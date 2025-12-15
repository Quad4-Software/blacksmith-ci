import type { VariantProps } from 'class-variance-authority';
import { cva } from 'class-variance-authority';

export { default as ToastAlert } from './ToastAlert.vue';

export const toastAlertVariants = cva(
  'relative w-full rounded-lg border px-4 py-3 text-sm grid grid-cols-[16px_1fr] gap-x-3 gap-y-0.5 items-start',
  {
    variants: {
      variant: {
        default: 'bg-background text-foreground border-border',
        success: 'bg-background text-foreground border-border [&>svg]:text-green-600 dark:[&>svg]:text-green-500',
        error: 'bg-background text-destructive border-destructive [&>svg]:text-destructive',
        warning: 'bg-background text-foreground border-border [&>svg]:text-amber-600 dark:[&>svg]:text-amber-500',
        info: 'bg-background text-foreground border-border [&>svg]:text-blue-600 dark:[&>svg]:text-blue-500',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  },
);

export type ToastAlertVariants = VariantProps<typeof toastAlertVariants>;

export const toastIcons = {
  success: 'lucide:circle-check',
  error: 'lucide:circle-x',
  warning: 'lucide:triangle-alert',
  info: 'lucide:info',
  default: 'lucide:info',
} as const;

export type ToastVariant = 'default' | 'success' | 'error' | 'warning' | 'info';
