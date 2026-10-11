import useConfig from '~/compositions/useConfig';
import useUserConfig from '~/compositions/useUserConfig';
import { useAuthStore } from '~/store/auth';

export default () => {
  const authStore = useAuthStore();

  return {
    // Return primitive boolean for backward compatibility
    // Components check `if (isAuthenticated)` directly
    get isAuthenticated() {
      return authStore.isAuthenticated;
    },

    // Return User | null for backward compatibility
    // Components access user.admin, user.org_id, etc. directly
    get user() {
      return authStore.user;
    },

    authenticateOIDC(url?: string) {
      if (url !== undefined) {
        const config = useUserConfig();
        config.setUserConfig('redirectUrl', url);
      }
      window.location.href = `${useConfig().rootPath}/authorize/oidc`;
    },

    authenticate(url?: string, forgeId?: number, link?: boolean, addAccount?: boolean) {
      if (url !== undefined) {
        const config = useUserConfig();
        config.setUserConfig('redirectUrl', url);
      }
      const params = new URLSearchParams();
      if (forgeId !== undefined) {
        params.set('forge_id', String(forgeId));
      }
      if (link) {
        params.set('link', 'true');
      }
      if (addAccount) {
        params.set('add_account', 'true');
      }
      const queryString = params.toString();
      window.location.href = `${useConfig().rootPath}/authorize${queryString ? `?${queryString}` : ''}`;
    },
  } as const;
};
