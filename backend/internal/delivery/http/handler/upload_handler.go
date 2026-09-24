package handler

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"lab-ap/pkg/response"
	"lab-ap/pkg/supabase"

	"github.com/gin-gonic/gin"
)

// maxUploadSize membatasi ukuran file unggahan (10 MB) untuk mencegah
// pemakaian memori berlebih (io.ReadAll memuat seluruh file ke RAM).
const maxUploadSize = 10 << 20 // 10 MiB

// CR-H1: batas & allowlist berkas. Folder hanya boleh satu segmen aman
// (mencegah `../../evil`), ekstensi+MIME dibatasi gambar/PDF, dan tipe isi
// disniff dari byte sebenarnya (header Content-Type dari klien tak dipercaya).
var (
	folderRe  = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	extToMime = map[string]string{
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".webp": "image/webp",
		".pdf":  "application/pdf",
	}
)

type UploadHandler struct {
	sb *supabase.Client
}

func NewUploadHandler(sb *supabase.Client) *UploadHandler { return &UploadHandler{sb: sb} }

// Upload POST /api/admin/upload (multipart: file, folder)
// @Summary Upload File
// @Description Mengunggah file ke Supabase Storage (gambar soal, dll)
// @Tags Admin - Upload
// @Security bearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "File yang diupload"
// @Param folder formData string false "Nama folder tujuan"
// @Success 201 {object} response.Envelope
// @Router /admin/upload [post]
func (h *UploadHandler) Upload(c *gin.Context) {
	if !h.sb.Enabled() {
		response.Fail(c, http.StatusServiceUnavailable, "Supabase Storage belum dikonfigurasi", nil)
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "File wajib diunggah (field 'file')", err.Error())
		return
	}
	if fileHeader.Size > maxUploadSize {
		response.Fail(c, http.StatusRequestEntityTooLarge, "Ukuran file melebihi batas 10 MB", nil)
		return
	}
	folder := strings.Trim(c.PostForm("folder"), "/")
	if folder == "" {
		folder = "uploads"
	}
	if !folderRe.MatchString(folder) {
		response.Fail(c, http.StatusBadRequest, "Nama folder tidak valid (hanya huruf, angka, _ dan -)", nil)
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "Gagal membuka file", err.Error())
		return
	}
	defer f.Close()
	// Batasi pembacaan secara defensif walau Size sudah dicek (header bisa berbohong).
	content, err := io.ReadAll(io.LimitReader(f, maxUploadSize+1))
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "Gagal membaca file", err.Error())
		return
	}
	if len(content) > maxUploadSize {
		response.Fail(c, http.StatusRequestEntityTooLarge, "Ukuran file melebihi batas 10 MB", nil)
		return
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	expectedMime, extOK := extToMime[ext]
	if !extOK {
		response.Fail(c, http.StatusBadRequest, "Ekstensi file tidak diizinkan (hanya .png, .jpg, .jpeg, .gif, .webp, .pdf)", nil)
		return
	}
	// Sniff isi sebenarnya; MIME dari klien tidak dipercaya.
	sniffed := http.DetectContentType(content)
	if i := strings.IndexByte(sniffed, ';'); i >= 0 {
		sniffed = strings.TrimSpace(sniffed[:i])
	}
	if sniffed != expectedMime {
		response.Fail(c, http.StatusBadRequest, "Isi file tidak cocok dengan ekstensinya (terdeteksi: "+sniffed+")", nil)
		return
	}

	name := fmt.Sprintf("%s/%d%s", folder, time.Now().UnixNano(), ext)

	// upsert=false: jangan pernah menimpa berkas yang sudah ada di bucket.
	url, err := h.sb.Upload(name, content, sniffed, false)
	if err != nil {
		response.Fail(c, http.StatusBadGateway, "Upload ke Supabase gagal", err.Error())
		return
	}
	response.Created(c, "File diunggah", gin.H{"url": url, "path": name})
}
