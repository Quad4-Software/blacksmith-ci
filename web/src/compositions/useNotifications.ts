import { h } from 'vue';
import { toast } from 'vue-sonner';

import type { ToastVariant } from '~/components/ui/toast';
import { ToastAlert } from '~/components/ui/toast';

interface NotifyOptions {
  title: string;
  text?: string;
  type?: 'success' | 'error' | 'warning' | 'info';
  duration?: number;
}

const DEFAULT_DURATION = 4000;

export const notify = ({ title, text, type, duration }: NotifyOptions) => {
  const variant: ToastVariant = type || 'default';

  toast.custom(
    () =>
      h(ToastAlert, {
        variant,
        title,
        description: text,
      }),
    {
      duration: duration === -1 ? Infinity : (duration ?? DEFAULT_DURATION),
    },
  );
};

export default () => ({ notify });
