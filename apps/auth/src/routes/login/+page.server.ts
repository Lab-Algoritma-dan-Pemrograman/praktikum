import type { Actions, ServerLoad } from '@sveltejs/kit';
import {
  centralLoginLoad,
  passwordLogin,
  startProviderLogin
} from '@algohub/auth-client/server';

export const load: ServerLoad = centralLoginLoad;
export const actions: Actions = {
  password: passwordLogin,
  oauth: startProviderLogin
};
