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

    authenticate(url?: string, forgeId?: number) {
      if (url !== undefined) {
        const config = useUserConfig();
        config.setUserConfig('redirectUrl', url);
      }
      window.location.href = `${useConfig().rootPath}/authorize?${forgeId !== undefined ? `forge_id=${forgeId}` : ''}`;
    },
  } as const;
};
