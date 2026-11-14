import { page } from '$app/state';

type Link = { name: string; origin: string };

/** Resolve an application host from the identity payload, never hardcoded. */
export function appLink(name: string): string {
  const applications = (page.data as { applications?: Link[] }).applications;
  return applications?.find((item) => item.name === name)?.origin ?? '/login';
}
