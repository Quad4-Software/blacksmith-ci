<template>
  <Settings :title="$t('user.settings.cli_and_api.cli_and_api')" :description="$t('user.settings.cli_and_api.desc')">
    <InputField :label="$t('user.settings.cli_and_api.cli_usage')">
      <template #headerActions>
        <a :href="cliDownload" target="_blank" class="ml-4 text-primary hover:text-primary/80">{{
          $t('user.settings.cli_and_api.download_cli')
        }}</a>
      </template>
      <pre class="code-box">{{ usageWithCli }}</pre>
    </InputField>
    <InputField :label="$t('user.settings.cli_and_api.token')">
      <template #titleActions>
        <Button class="ml-auto" @click="resetToken">{{ $t('user.settings.cli_and_api.reset_token') }}</Button>
      </template>
      <pre class="code-box">{{ token }}</pre>
    </InputField>
    <InputField :label="$t('user.settings.cli_and_api.api_usage')">
      <template #headerActions>
        <a
          v-if="enableSwagger"
          :href="`${address}/swagger/index.html`"
          target="_blank"
          class="ml-4 text-primary hover:text-primary/80"
        >
          {{ $t('user.settings.cli_and_api.swagger_ui') }}
        </a>
      </template>
      <pre class="code-box">{{ usageWithCurl }}</pre>
    </InputField>
  </Settings>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import { Button } from '~/components/ui/button';
import InputField from '~/components/form/InputField.vue';
import Settings from '~/components/layout/Settings.vue';
import useApiClient from '~/compositions/useApiClient';
import useConfig from '~/compositions/useConfig';
import { useCrowTitle } from '~/compositions/useCrowTitle';

const { rootPath, enableSwagger } = useConfig();

const apiClient = useApiClient();
const token = ref<string | undefined>();

onMounted(async () => {
  token.value = await apiClient.getToken();
});

const address = `${window.location.protocol}//${window.location.host}${rootPath}`; // port is included in location.host

const usageWithCurl = computed(() => {
  let usage = `export CROW_SERVER="${address}"\n`;
  usage += `export CROW_TOKEN="${token.value}"\n`;
  usage += `\n`;
  usage += `# curl -i \${CROW_SERVER}/api/user -H "Authorization: Bearer \${CROW_TOKEN}"`;
  return usage;
});

const usageWithCli = `# crow-cli setup --server ${address}`;

const cliDownload = 'https://codeberg.org/crowci/crow/releases';

const resetToken = async () => {
  token.value = await apiClient.resetToken();
  window.location.href = `${address}/logout`;
};

const { t } = useI18n();
useCrowTitle(computed(() => [t('user.settings.cli_and_api.cli_and_api'), t('user.settings.settings')]));
</script>
