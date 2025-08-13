import '~/compositions/useFavicon';
import '~/tailwind.css';
import '~/style.css';

import { createPinia } from 'pinia';
import { createApp } from 'vue';

import App from '~/App.vue';
import useEvents from '~/compositions/useEvents';
import { useForges } from '~/compositions/useForges';
import { i18n } from '~/compositions/useI18n';
import { notifications } from '~/compositions/useNotifications';
import router from '~/router';

const app = createApp(App);

// eslint-disable-next-line ts/no-unsafe-argument
app.use(router as any);
// eslint-disable-next-line ts/no-unsafe-argument
app.use(notifications as any);
app.use(i18n);

// eslint-disable-next-line ts/no-unsafe-argument
app.use(createPinia() as any);
app.mount('#app');

useEvents();

// Preload forge data globally
const { loadForges } = useForges();
void loadForges();
