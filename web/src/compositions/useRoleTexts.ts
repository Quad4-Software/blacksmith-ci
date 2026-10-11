import { computed } from 'vue';
import { useI18n } from 'vue-i18n';

import type { InstanceRole } from '~/lib/api/types';

// Translated name and description of every instance role.
export default () => {
  const { t } = useI18n();

  return computed<Record<InstanceRole, { label: string; description: string }>>(() => ({
    operator: {
      label: t('admin.settings.users.role_operator'),
      description: t('admin.settings.users.role_operator_desc'),
    },
    'secrets-manager': {
      label: t('admin.settings.users.role_secrets_manager'),
      description: t('admin.settings.users.role_secrets_manager_desc'),
    },
    auditor: {
      label: t('admin.settings.users.role_auditor'),
      description: t('admin.settings.users.role_auditor_desc'),
    },
  }));
};
