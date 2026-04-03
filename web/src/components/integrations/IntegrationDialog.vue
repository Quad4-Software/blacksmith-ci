<template>
  <Dialog v-model:open="localOpen">
    <DialogTrigger as-child>
      <slot name="trigger" />
    </DialogTrigger>
    <DialogContent class="sm:max-w-2xl max-h-[90vh] overflow-y-auto">
      <DialogHeader>
        <DialogTitle>
          {{ isEditing ? $t('integrations.edit') : $t('integrations.add') }}
        </DialogTitle>
        <DialogDescription>
          {{ $t('integrations.dialog_description') }}
        </DialogDescription>
      </DialogHeader>

      <form class="space-y-4" @submit.prevent="handleSave">
        <div class="space-y-2">
          <Label for="name">{{ $t('integrations.name') }}</Label>
          <Input id="name" v-model="form.name" required :placeholder="$t('integrations.name_placeholder')" />
        </div>

        <div class="space-y-2">
          <Label for="type">{{ $t('integrations.type') }}</Label>
          <Select v-model="form.type" :disabled="isEditing">
            <SelectTrigger>
              <SelectValue :placeholder="$t('integrations.type_placeholder')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="vault">{{ $t('integrations.type_vault') }}</SelectItem>
              <SelectItem value="azure_key_vault">{{ $t('integrations.type_azure_key_vault') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div class="space-y-2">
          <Label for="description">{{ $t('integrations.description') }}</Label>
          <Textarea
            id="description"
            v-model="form.description"
            :placeholder="$t('integrations.description_placeholder')"
            rows="2"
          />
        </div>

        <!-- Vault Configuration -->
        <div v-if="form.type === 'vault'" class="space-y-4 p-4 border rounded-lg">
          <h4 class="font-medium">{{ $t('integrations.vault_config') }}</h4>

          <div class="space-y-2">
            <Label for="vault-address">{{ $t('integrations.vault_address') }}</Label>
            <Input
              id="vault-address"
              v-model="vaultConfig.address"
              required
              placeholder="https://vault.example.com:8200"
            />
          </div>

          <div class="grid grid-cols-2 gap-4">
            <div class="space-y-2">
              <Label for="vault-role-id">{{ $t('integrations.vault_role_id') }}</Label>
              <Input id="vault-role-id" v-model="vaultConfig.role_id" required placeholder="role-id" />
            </div>
            <div class="space-y-2">
              <Label for="vault-secret-id">{{ $t('integrations.vault_secret_id') }}</Label>
              <Input
                id="vault-secret-id"
                v-model="vaultConfig.secret_id"
                type="password"
                :placeholder="isEditing ? $t('integrations.vault_secret_id_unchanged') : 'secret-id'"
              />
            </div>
          </div>

          <div class="grid grid-cols-2 gap-4">
            <div class="space-y-2">
              <Label for="vault-auth-path">{{ $t('integrations.vault_auth_path') }}</Label>
              <Input id="vault-auth-path" v-model="vaultConfig.auth_path" placeholder="approle" />
              <p class="text-xs text-muted-foreground">{{ $t('integrations.vault_auth_path_help') }}</p>
            </div>
            <div class="space-y-2">
              <Label for="vault-mount-path">{{ $t('integrations.vault_mount_path') }}</Label>
              <Input id="vault-mount-path" v-model="vaultConfig.mount_path" placeholder="secret" />
              <p class="text-xs text-muted-foreground">{{ $t('integrations.vault_mount_path_help') }}</p>
            </div>
          </div>

          <div class="space-y-2">
            <Label for="vault-namespace">{{ $t('integrations.vault_namespace') }}</Label>
            <Input id="vault-namespace" v-model="vaultConfig.namespace" placeholder="admin" />
            <p class="text-xs text-muted-foreground">{{ $t('integrations.vault_namespace_help') }}</p>
          </div>

          <div class="flex items-center space-x-2">
            <Checkbox id="vault-tls-skip" v-model:checked="vaultConfig.tls_skip_verify" />
            <Label for="vault-tls-skip" class="text-sm font-normal">
              {{ $t('integrations.vault_tls_skip_verify') }}
            </Label>
          </div>

          <div class="space-y-2">
            <Label for="vault-ca-cert">{{ $t('integrations.vault_ca_certificate') }}</Label>
            <Textarea
              id="vault-ca-cert"
              v-model="vaultConfig.ca_certificate"
              :placeholder="$t('integrations.vault_ca_certificate_placeholder')"
              rows="3"
              class="font-mono text-xs"
            />
          </div>
        </div>

        <!-- Azure Key Vault Configuration -->
        <div v-if="form.type === 'azure_key_vault'" class="space-y-4 p-4 border rounded-lg">
          <h4 class="font-medium">{{ $t('integrations.azure_kv_config') }}</h4>

          <div class="space-y-2">
            <Label for="azure-vault-url">{{ $t('integrations.azure_kv_vault_url') }}</Label>
            <Input
              id="azure-vault-url"
              v-model="azureKeyVaultConfig.vault_url"
              required
              placeholder="https://my-vault.vault.azure.net"
            />
          </div>

          <div class="space-y-2">
            <Label for="azure-tenant-id">{{ $t('integrations.azure_kv_tenant_id') }}</Label>
            <Input
              id="azure-tenant-id"
              v-model="azureKeyVaultConfig.tenant_id"
              required
              placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
            />
          </div>

          <div class="grid grid-cols-2 gap-4">
            <div class="space-y-2">
              <Label for="azure-client-id">{{ $t('integrations.azure_kv_client_id') }}</Label>
              <Input
                id="azure-client-id"
                v-model="azureKeyVaultConfig.client_id"
                required
                placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
              />
            </div>
            <div class="space-y-2">
              <Label for="azure-client-secret">{{ $t('integrations.azure_kv_client_secret') }}</Label>
              <Input
                id="azure-client-secret"
                v-model="azureKeyVaultConfig.client_secret"
                type="password"
                :required="!isEditing"
                :placeholder="isEditing ? $t('integrations.azure_kv_client_secret_unchanged') : 'client-secret'"
              />
            </div>
          </div>
        </div>

        <!-- Access Control / Allowlists -->
        <div class="space-y-4 p-4 border rounded-lg">
          <h4 class="font-medium">{{ $t('integrations.access_control') }}</h4>
          <p class="text-sm text-muted-foreground">{{ $t('integrations.access_control_desc') }}</p>

          <AllowlistSelector
            :model-value="form.allowed_user_ids ?? []"
            :items="availableUsers"
            :label="$t('integrations.allowed_users')"
            :placeholder="$t('integrations.select_users')"
            :search-placeholder="$t('integrations.search_users')"
            :help-text="$t('integrations.allowed_users_help')"
            :loading="loadingUsers"
            @update:model-value="form.allowed_user_ids = $event"
          />

          <AllowlistSelector
            :model-value="form.allowed_org_ids ?? []"
            :items="availableOrgs"
            :label="$t('integrations.allowed_orgs')"
            :placeholder="$t('integrations.select_orgs')"
            :search-placeholder="$t('integrations.search_orgs')"
            :help-text="$t('integrations.allowed_orgs_help')"
            :loading="loadingOrgs"
            @update:model-value="form.allowed_org_ids = $event"
          />

          <AllowlistSelector
            :model-value="form.allowed_repo_ids ?? []"
            :items="availableRepos"
            :label="$t('integrations.allowed_repos')"
            :placeholder="$t('integrations.select_repos')"
            :search-placeholder="$t('integrations.search_repos')"
            :help-text="$t('integrations.allowed_repos_help')"
            :loading="loadingRepos"
            @update:model-value="form.allowed_repo_ids = $event"
          />
        </div>

        <div class="flex items-center space-x-2">
          <Checkbox id="is-active" v-model:checked="form.is_active" />
          <Label for="is-active" class="text-sm font-normal">
            {{ $t('integrations.is_active') }}
          </Label>
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" @click="handleCancel">
            {{ $t('cancel') }}
          </Button>
          <Button type="submit" :disabled="isSaving">
            {{ isSaving ? $t('saving') : $t('save') }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue';

import type { AllowlistItem } from './AllowlistSelector.vue';
import AllowlistSelector from './AllowlistSelector.vue';
import { Button } from '~/components/ui/button';
import { Checkbox } from '~/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '~/components/ui/dialog';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { Textarea } from '~/components/ui/textarea';
import useApiClient from '~/compositions/useApiClient';
import type { AzureKeyVaultConfig, Integration, IntegrationType, VaultConfig } from '~/lib/api/types';

const props = defineProps<{
  open: boolean;
  integration?: Partial<Integration>;
  isSaving: boolean;
}>();

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void;
  (e: 'save', integration: Partial<Integration>): void;
  (e: 'cancel'): void;
}>();

const apiClient = useApiClient();

const localOpen = computed({
  get: () => props.open,
  set: (value) => emit('update:open', value),
});

const isEditing = computed(() => !!props.integration?.id);

const form = ref<Partial<Integration>>({
  name: '',
  type: 'vault' as IntegrationType,
  description: '',
  is_active: true,
  allowed_user_ids: [],
  allowed_org_ids: [],
  allowed_repo_ids: [],
});

const vaultConfig = ref<VaultConfig>({
  address: '',
  role_id: '',
  secret_id: '',
  auth_path: '',
  mount_path: '',
  namespace: '',
  tls_skip_verify: false,
  ca_certificate: '',
});

const azureKeyVaultConfig = ref<AzureKeyVaultConfig>({
  vault_url: '',
  tenant_id: '',
  client_id: '',
  client_secret: '',
});

// Allowlist data
const availableUsers = ref<AllowlistItem[]>([]);
const availableOrgs = ref<AllowlistItem[]>([]);
const availableRepos = ref<AllowlistItem[]>([]);
const loadingUsers = ref(false);
const loadingOrgs = ref(false);
const loadingRepos = ref(false);

// Load allowlist options when dialog opens
watch(
  () => props.open,
  async (isOpen) => {
    if (isOpen && availableUsers.value.length === 0) {
      await loadAllowlistOptions();
    }
  },
);

async function loadAllowlistOptions() {
  // Load users
  loadingUsers.value = true;
  try {
    const users = await apiClient.getUsers();
    availableUsers.value = (users || []).map((u) => ({
      id: u.id,
      name: u.login,
      description: u.email,
    }));
  } catch {
    availableUsers.value = [];
  } finally {
    loadingUsers.value = false;
  }

  // Load orgs (user's orgs)
  loadingOrgs.value = true;
  try {
    const orgs = await apiClient.getUserOrgs();
    availableOrgs.value = (orgs || []).map((o) => ({
      id: o.id,
      name: o.name,
    }));
  } catch {
    availableOrgs.value = [];
  } finally {
    loadingOrgs.value = false;
  }

  // Load repos (user's repos)
  loadingRepos.value = true;
  try {
    const repos = await apiClient.getRepoList();
    availableRepos.value = (repos || []).map((r) => ({
      id: r.id,
      name: r.full_name,
    }));
  } catch {
    availableRepos.value = [];
  } finally {
    loadingRepos.value = false;
  }
}

// Watch for integration changes to populate the form
watch(
  () => props.integration,
  (newIntegration) => {
    if (newIntegration) {
      form.value = {
        id: newIntegration.id,
        name: newIntegration.name || '',
        type: newIntegration.type || 'vault',
        description: newIntegration.description || '',
        is_active: newIntegration.is_active ?? true,
        allowed_user_ids: newIntegration.allowed_user_ids || [],
        allowed_org_ids: newIntegration.allowed_org_ids || [],
        allowed_repo_ids: newIntegration.allowed_repo_ids || [],
      };

      // Parse existing config if available
      if (newIntegration.config && newIntegration.type === 'vault') {
        try {
          const parsed = JSON.parse(newIntegration.config) as VaultConfig;
          vaultConfig.value = {
            address: parsed.address || '',
            role_id: parsed.role_id || '',
            secret_id: '', // Don't show existing secret_id
            auth_path: parsed.auth_path || '',
            mount_path: parsed.mount_path || '',
            namespace: parsed.namespace || '',
            tls_skip_verify: parsed.tls_skip_verify || false,
            ca_certificate: parsed.ca_certificate || '',
          };
        } catch {
          resetVaultConfig();
        }
      } else {
        resetVaultConfig();
      }

      if (newIntegration.config && newIntegration.type === 'azure_key_vault') {
        try {
          const parsed = JSON.parse(newIntegration.config) as AzureKeyVaultConfig;
          azureKeyVaultConfig.value = {
            vault_url: parsed.vault_url || '',
            tenant_id: parsed.tenant_id || '',
            client_id: parsed.client_id || '',
            client_secret: '', // Don't show existing client_secret
          };
        } catch {
          resetAzureKeyVaultConfig();
        }
      } else {
        resetAzureKeyVaultConfig();
      }
    } else {
      resetForm();
    }
  },
  { immediate: true },
);

function resetVaultConfig() {
  vaultConfig.value = {
    address: '',
    role_id: '',
    secret_id: '',
    auth_path: '',
    mount_path: '',
    namespace: '',
    tls_skip_verify: false,
    ca_certificate: '',
  };
}

function resetAzureKeyVaultConfig() {
  azureKeyVaultConfig.value = {
    vault_url: '',
    tenant_id: '',
    client_id: '',
    client_secret: '',
  };
}

function resetForm() {
  form.value = {
    name: '',
    type: 'vault',
    description: '',
    is_active: true,
    allowed_user_ids: [],
    allowed_org_ids: [],
    allowed_repo_ids: [],
  };
  resetVaultConfig();
  resetAzureKeyVaultConfig();
}

function handleSave() {
  // Build the config JSON based on type
  let config = '';
  if (form.value.type === 'vault') {
    const configObj: Record<string, unknown> = {
      address: vaultConfig.value.address,
      role_id: vaultConfig.value.role_id,
    };

    // Only include secret_id if provided (for new or if user wants to update)
    if (vaultConfig.value.secret_id) {
      configObj.secret_id = vaultConfig.value.secret_id;
    }

    // Add mount_path (required, with default)
    configObj.mount_path = vaultConfig.value.mount_path || 'secret';

    // Add optional fields if set
    if (vaultConfig.value.auth_path) {
      configObj.auth_path = vaultConfig.value.auth_path;
    }
    if (vaultConfig.value.namespace) {
      configObj.namespace = vaultConfig.value.namespace;
    }
    if (vaultConfig.value.tls_skip_verify) {
      configObj.tls_skip_verify = vaultConfig.value.tls_skip_verify;
    }
    if (vaultConfig.value.ca_certificate) {
      configObj.ca_certificate = vaultConfig.value.ca_certificate;
    }

    config = JSON.stringify(configObj);
  } else if (form.value.type === 'azure_key_vault') {
    const configObj: Record<string, unknown> = {
      vault_url: azureKeyVaultConfig.value.vault_url,
      tenant_id: azureKeyVaultConfig.value.tenant_id,
      client_id: azureKeyVaultConfig.value.client_id,
    };

    // Only include client_secret if provided (for new or if user wants to update)
    if (azureKeyVaultConfig.value.client_secret) {
      configObj.client_secret = azureKeyVaultConfig.value.client_secret;
    }

    config = JSON.stringify(configObj);
  }

  emit('save', {
    ...form.value,
    config,
  });
}

function handleCancel() {
  emit('cancel');
}
</script>
