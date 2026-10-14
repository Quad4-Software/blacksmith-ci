// JetBrains Mono is bundled so the UI works without access to a font CDN
import '@fontsource/jetbrains-mono/400.css';
import '~/compositions/useFavicon';
import '~/tailwind.css';
import '~/style.css';

import { createPinia } from 'pinia';
import { createApp } from 'vue';

import App from '~/App.vue';
import useApiClient from '~/compositions/useApiClient';
import useEvents from '~/compositions/useEvents';
import { useForges } from '~/compositions/useForges';
import { i18n } from '~/compositions/useI18n';
import router from '~/router';
import { useAuthStore } from '~/store/auth';

const app = createApp(App);

// eslint-disable-next-line ts/no-unsafe-argument
app.use(router as any);
app.use(i18n);

// eslint-disable-next-line ts/no-unsafe-argument
app.use(createPinia() as any);
app.mount('#app');

// Initialize auth store and connect to API client for session management
const authStore = useAuthStore();
const apiClient = useApiClient();
apiClient.setAuthStore(authStore);

// Only subscribe to SSE events for authenticated users
// Anonymous users don't need real-time updates and would get 401 errors
if (authStore.isAuthenticated) {
  useEvents();
}

// Preload forge data globally
const { loadForges } = useForges();
void loadForges();
