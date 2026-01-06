<template>
  <Settings :title="$t('user.settings.general.general')">
    <!-- Profile Section -->
    <div class="space-y-4">
      <div>
        <h3 class="text-lg text-foreground">{{ $t('user.settings.general.section_profile') }}</h3>
        <p class="text-sm text-muted-foreground">
          {{ $t('user.settings.general.section_profile_desc') }}
        </p>
      </div>
      <div class="flex flex-col md:flex-row gap-6 flex-wrap">
        <div>
          <InputField v-slot="{ id }" :label="$t('user.settings.general.description')">
            <Textarea
              :id="id"
              v-model="userDescription"
              :placeholder="$t('user.settings.general.description_placeholder')"
              rows="3"
              class="resize-none w-full md:w-[250px]"
            />
          </InputField>
        </div>
        <div>
          <InputField v-slot="{ id }" :label="$t('user.settings.general.timezone')">
            <Select :id="id" v-model="userTimezone">
              <SelectTrigger>
                <SelectValue :placeholder="$t('user.settings.general.timezone_placeholder')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="tz in timezoneOptions" :key="tz" :value="tz">
                  {{ tz }}
                </SelectItem>
              </SelectContent>
            </Select>
          </InputField>
        </div>
        <div>
          <InputField v-slot="{ id }" :label="$t('user.settings.general.language')">
            <Select :id="id" v-model="selectedLocale">
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="option in localeOptions" :key="option.value" :value="option.value">
                  {{ option.text }}
                </SelectItem>
              </SelectContent>
            </Select>
          </InputField>
        </div>
      </div>
      <Button @click="saveProfile">
        <Icon v-if="isSaving" name="loading" class="mr-2 h-4 w-4 animate-spin" />
        {{ $t('user.settings.general.save') }}
      </Button>
    </div>

    <Separator class="my-6" />

    <!-- Appearance Section -->
    <div class="space-y-4">
      <div>
        <h3 class="text-lg text-foreground">{{ $t('user.settings.general.section_appearance') }}</h3>
        <p class="text-sm text-muted-foreground">
          {{ $t('user.settings.general.section_appearance_desc') }}
        </p>
      </div>
      <div class="flex flex-col md:flex-row gap-6 flex-wrap">
        <div>
          <InputField v-slot="{ id }" :label="$t('user.settings.general.theme_label')">
            <Select :id="id" v-model="themeName">
              <SelectTrigger class="w-full md:w-[200px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="theme in availableThemes" :key="theme.id" :value="theme.id">
                  {{ theme.name }}
                </SelectItem>
              </SelectContent>
            </Select>
          </InputField>
        </div>
        <div>
          <InputField v-slot="{ id }" :label="$t('user.settings.general.color_mode')">
            <Select :id="id" v-model="storeTheme">
              <SelectTrigger class="w-full md:w-[200px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="option in colorModeOptions" :key="option.value" :value="option.value">
                  {{ option.text }}
                </SelectItem>
              </SelectContent>
            </Select>
          </InputField>
        </div>
        <div>
          <InputField v-slot="{ id }" :label="$t('user.settings.general.font_size')">
            <Select :id="id" v-model="fontSize">
              <SelectTrigger class="w-full md:w-[200px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="option in FONT_SIZE_OPTIONS" :key="option.value" :value="option.value">
                  {{ option.label }}
                </SelectItem>
              </SelectContent>
            </Select>
          </InputField>
        </div>
      </div>
    </div>

    <Separator class="my-6" />

    <!-- Pipeline View Section -->
    <div class="space-y-4">
      <div>
        <h3 class="text-lg text-foreground">{{ $t('user.settings.general.section_pipeline_view') }}</h3>
        <p class="text-sm text-muted-foreground">
          {{ $t('user.settings.general.section_pipeline_view_desc') }}
        </p>
      </div>
      <div class="flex flex-col md:flex-row gap-6 flex-wrap">
        <div>
          <InputField :label="$t('user.settings.general.pipeline_steps_width')">
            <div class="flex items-center gap-2">
              <NumberField v-model="pipelineSplitRatioInput" :min="25" :max="50" class="max-w-[8rem]">
                <NumberFieldContent>
                  <NumberFieldDecrement />
                  <NumberFieldInput />
                  <NumberFieldIncrement />
                </NumberFieldContent>
              </NumberField>
              <span class="text-sm text-muted-foreground">
                {{ $t('user.settings.general.split_ratio_range') }}
              </span>
            </div>
          </InputField>
        </div>

        <div>
          <InputField v-slot="{ id }" :label="$t('user.settings.general.log_theme')">
            <Select :id="id" v-model="logTheme">
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="option in logThemeOptions" :key="option.value" :value="option.value">
                  {{ option.text }}
                </SelectItem>
              </SelectContent>
            </Select>
          </InputField>
        </div>

        <div class="mb-4 mt-2 flex flex-col">
          <label class="mb-2 text-foreground">{{ $t('user.settings.general.auto_follow') }}</label>
          <div class="flex items-center gap-3">
            <Switch v-model="autoFollowEnabled" />
            <span class="text-sm text-muted-foreground">
              {{ $t('user.settings.general.auto_follow_desc') }}
            </span>
          </div>
        </div>
      </div>
    </div>
  </Settings>
</template>

<script lang="ts" setup>
import { useStorage } from '@vueuse/core';
import { SUPPORTED_LOCALES } from 'virtual:vue-i18n-supported-locales';
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import InputField from '~/components/form/InputField.vue';
import {
  NumberField,
  NumberFieldContent,
  NumberFieldDecrement,
  NumberFieldIncrement,
  NumberFieldInput,
} from '~/components/ui/number-field';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { Separator } from '~/components/ui/separator';
import { Switch } from '~/components/ui/switch';
import { Textarea } from '~/components/ui/textarea';
import { Button } from '~/components/ui/button';
import Settings from '~/components/layout/Settings.vue';
import { setI18nLanguage } from '~/compositions/useI18n';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import { FONT_SIZE_OPTIONS, useTheme } from '~/compositions/useTheme';
import useApiClient from '~/compositions/useApiClient';
import useAuthentication from '~/compositions/useAuthentication';
import useNotifications from '~/compositions/useNotifications';

const { locale, t } = useI18n();
const apiClient = useApiClient();
const { user } = useAuthentication();
const notifications = useNotifications();

// Theme settings
const { themeName, availableThemes, storeTheme, fontSize } = useTheme();

const colorModeOptions = computed(() => [
  { value: 'light', text: t('user.settings.general.theme.light') },
  { value: 'dark', text: t('user.settings.general.theme.dark') },
  { value: 'auto', text: t('user.settings.general.theme.auto') },
]);

// Profile fields
const userDescription = ref(user?.description || '');
const userTimezone = ref(user?.timezone || '');
const isSaving = ref(false);

// Load timezones from API
const timezoneOptions = ref<string[]>([]);

onMounted(async () => {
  try {
    const zones = await apiClient.getTimezones();
    timezoneOptions.value = zones;
  } catch {
    // Fallback to a basic list if API fails
    timezoneOptions.value = [
      'UTC',
      'Africa/Abidjan',
      'Africa/Accra',
      'Africa/Addis_Ababa',
      'Africa/Algiers',
      'Africa/Asmara',
      'Africa/Bamako',
      'Africa/Bangui',
      'Africa/Banjul',
      'Africa/Bissau',
      'Africa/Blantyre',
      'Africa/Brazzaville',
      'Africa/Bujumbura',
      'Africa/Cairo',
      'Africa/Casablanca',
      'Africa/Ceuta',
      'Africa/Conakry',
      'Africa/Dakar',
      'Africa/Dar_es_Salaam',
      'Africa/Djibouti',
      'Africa/Douala',
      'Africa/El_Aaiun',
      'Africa/Freetown',
      'Africa/Gaborone',
      'Africa/Harare',
      'Africa/Johannesburg',
      'Africa/Juba',
      'Africa/Kampala',
      'Africa/Khartoum',
      'Africa/Kigali',
      'Africa/Kinshasa',
      'Africa/Lagos',
      'Africa/Libreville',
      'Africa/Lome',
      'Africa/Luanda',
      'Africa/Lubumbashi',
      'Africa/Lusaka',
      'Africa/Malabo',
      'Africa/Maputo',
      'Africa/Maseru',
      'Africa/Mbabane',
      'Africa/Mogadishu',
      'Africa/Monrovia',
      'Africa/Nairobi',
      'Africa/Ndjamena',
      'Africa/Niamey',
      'Africa/Nouakchott',
      'Africa/Ouagadougou',
      'Africa/Porto-Novo',
      'Africa/Sao_Tome',
      'Africa/Tripoli',
      'Africa/Tunis',
      'Africa/Windhoek',
      'America/Adak',
      'America/Anchorage',
      'America/Anguilla',
      'America/Antigua',
      'America/Araguaina',
      'America/Argentina/Buenos_Aires',
      'America/Argentina/Catamarca',
      'America/Argentina/Cordoba',
      'America/Argentina/Jujuy',
      'America/Argentina/La_Rioja',
      'America/Argentina/Mendoza',
      'America/Argentina/Rio_Gallegos',
      'America/Argentina/Salta',
      'America/Argentina/San_Juan',
      'America/Argentina/San_Luis',
      'America/Argentina/Tucuman',
      'America/Argentina/Ushuaia',
      'America/Aruba',
      'America/Asuncion',
      'America/Atikokan',
      'America/Bahia',
      'America/Bahia_Banderas',
      'America/Barbados',
      'America/Belem',
      'America/Belize',
      'America/Blanc-Sablon',
      'America/Boa_Vista',
      'America/Bogota',
      'America/Boise',
      'America/Cambridge_Bay',
      'America/Campo_Grande',
      'America/Cancun',
      'America/Caracas',
      'America/Cayenne',
      'America/Cayman',
      'America/Chicago',
      'America/Chihuahua',
      'America/Costa_Rica',
      'America/Creston',
      'America/Cuiaba',
      'America/Curacao',
      'America/Danmarkshavn',
      'America/Dawson',
      'America/Dawson_Creek',
      'America/Denver',
      'America/Detroit',
      'America/Dominica',
      'America/Edmonton',
      'America/Eirunepe',
      'America/El_Salvador',
      'America/Fort_Nelson',
      'America/Fortaleza',
      'America/Glace_Bay',
      'America/Godthab',
      'America/Goose_Bay',
      'America/Grand_Turk',
      'America/Grenada',
      'America/Guadeloupe',
      'America/Guatemala',
      'America/Guayaquil',
      'America/Guyana',
      'America/Halifax',
      'America/Havana',
      'America/Hermosillo',
      'America/Indiana/Indianapolis',
      'America/Indiana/Knox',
      'America/Indiana/Marengo',
      'America/Indiana/Petersburg',
      'America/Indiana/Tell_City',
      'America/Indiana/Vevay',
      'America/Indiana/Vincennes',
      'America/Indiana/Winamac',
      'America/Inuvik',
      'America/Iqaluit',
      'America/Jamaica',
      'America/Juneau',
      'America/Kentucky/Louisville',
      'America/Kentucky/Monticello',
      'America/Kralendijk',
      'America/La_Paz',
      'America/Lima',
      'America/Los_Angeles',
      'America/Lower_Princes',
      'America/Maceio',
      'America/Managua',
      'America/Manaus',
      'America/Marigot',
      'America/Martinique',
      'America/Matamoros',
      'America/Mazatlan',
      'America/Menominee',
      'America/Merida',
      'America/Metlakatla',
      'America/Mexico_City',
      'America/Miquelon',
      'America/Moncton',
      'America/Monterrey',
      'America/Montevideo',
      'America/Montserrat',
      'America/Nassau',
      'America/New_York',
      'America/Nipigon',
      'America/Nome',
      'America/Noronha',
      'America/North_Dakota/Beulah',
      'America/North_Dakota/Center',
      'America/North_Dakota/New_Salem',
      'America/Ojinaga',
      'America/Panama',
      'America/Pangnirtung',
      'America/Paramaribo',
      'America/Phoenix',
      'America/Port-au-Prince',
      'America/Port_of_Spain',
      'America/Porto_Velho',
      'America/Puerto_Rico',
      'America/Punta_Arenas',
      'America/Rainy_River',
      'America/Rankin_Inlet',
      'America/Recife',
      'America/Regina',
      'America/Resolute',
      'America/Rio_Branco',
      'America/Santarem',
      'America/Santiago',
      'America/Santo_Domingo',
      'America/Sao_Paulo',
      'America/Scoresbysund',
      'America/Sitka',
      'America/St_Barthelemy',
      'America/St_Johns',
      'America/St_Kitts',
      'America/St_Lucia',
      'America/St_Thomas',
      'America/St_Vincent',
      'America/Swift_Current',
      'America/Tegucigalpa',
      'America/Thule',
      'America/Thunder_Bay',
      'America/Tijuana',
      'America/Toronto',
      'America/Tortola',
      'America/Vancouver',
      'America/Whitehorse',
      'America/Winnipeg',
      'America/Yakutat',
      'America/Yellowknife',
      'Antarctica/Casey',
      'Antarctica/Davis',
      'Antarctica/DumontDUrville',
      'Antarctica/Macquarie',
      'Antarctica/Mawson',
      'Antarctica/McMurdo',
      'Antarctica/Palmer',
      'Antarctica/Rothera',
      'Antarctica/Syowa',
      'Antarctica/Troll',
      'Antarctica/Vostok',
      'Arctic/Longyearbyen',
      'Asia/Aden',
      'Asia/Almaty',
      'Asia/Amman',
      'Asia/Anadyr',
      'Asia/Aqtau',
      'Asia/Aqtobe',
      'Asia/Ashgabat',
      'Asia/Atyrau',
      'Asia/Baghdad',
      'Asia/Bahrain',
      'Asia/Baku',
      'Asia/Bangkok',
      'Asia/Barnaul',
      'Asia/Beirut',
      'Asia/Bishkek',
      'Asia/Brunei',
      'Asia/Chita',
      'Asia/Choibalsan',
      'Asia/Colombo',
      'Asia/Damascus',
      'Asia/Dhaka',
      'Asia/Dili',
      'Asia/Dubai',
      'Asia/Dushanbe',
      'Asia/Famagusta',
      'Asia/Gaza',
      'Asia/Hebron',
      'Asia/Ho_Chi_Minh',
      'Asia/Hong_Kong',
      'Asia/Hovd',
      'Asia/Irkutsk',
      'Asia/Jakarta',
      'Asia/Jayapura',
      'Asia/Jerusalem',
      'Asia/Kabul',
      'Asia/Kamchatka',
      'Asia/Karachi',
      'Asia/Kathmandu',
      'Asia/Khandyga',
      'Asia/Kolkata',
      'Asia/Krasnoyarsk',
      'Asia/Kuala_Lumpur',
      'Asia/Kuching',
      'Asia/Kuwait',
      'Asia/Macau',
      'Asia/Magadan',
      'Asia/Makassar',
      'Asia/Manila',
      'Asia/Muscat',
      'Asia/Nicosia',
      'Asia/Novokuznetsk',
      'Asia/Novosibirsk',
      'Asia/Omsk',
      'Asia/Oral',
      'Asia/Phnom_Penh',
      'Asia/Pontianak',
      'Asia/Pyongyang',
      'Asia/Qatar',
      'Asia/Qostanay',
      'Asia/Qyzylorda',
      'Asia/Riyadh',
      'Asia/Sakhalin',
      'Asia/Samarkand',
      'Asia/Seoul',
      'Asia/Shanghai',
      'Asia/Singapore',
      'Asia/Srednekolymsk',
      'Asia/Taipei',
      'Asia/Tashkent',
      'Asia/Tbilisi',
      'Asia/Tehran',
      'Asia/Thimphu',
      'Asia/Tokyo',
      'Asia/Tomsk',
      'Asia/Ulaanbaatar',
      'Asia/Urumqi',
      'Asia/Ust-Nera',
      'Asia/Vientiane',
      'Asia/Vladivostok',
      'Asia/Yakutsk',
      'Asia/Yangon',
      'Asia/Yekaterinburg',
      'Asia/Yerevan',
      'Atlantic/Azores',
      'Atlantic/Bermuda',
      'Atlantic/Canary',
      'Atlantic/Cape_Verde',
      'Atlantic/Faroe',
      'Atlantic/Madeira',
      'Atlantic/Reykjavik',
      'Atlantic/South_Georgia',
      'Atlantic/St_Helena',
      'Atlantic/Stanley',
      'Australia/Adelaide',
      'Australia/Brisbane',
      'Australia/Broken_Hill',
      'Australia/Currie',
      'Australia/Darwin',
      'Australia/Eucla',
      'Australia/Hobart',
      'Australia/Lindeman',
      'Australia/Lord_Howe',
      'Australia/Melbourne',
      'Australia/Perth',
      'Australia/Sydney',
      'Europe/Amsterdam',
      'Europe/Andorra',
      'Europe/Astrakhan',
      'Europe/Athens',
      'Europe/Belgrade',
      'Europe/Berlin',
      'Europe/Bratislava',
      'Europe/Brussels',
      'Europe/Bucharest',
      'Europe/Budapest',
      'Europe/Busingen',
      'Europe/Chisinau',
      'Europe/Copenhagen',
      'Europe/Dublin',
      'Europe/Gibraltar',
      'Europe/Guernsey',
      'Europe/Helsinki',
      'Europe/Isle_of_Man',
      'Europe/Istanbul',
      'Europe/Jersey',
      'Europe/Kaliningrad',
      'Europe/Kiev',
      'Europe/Kirov',
      'Europe/Lisbon',
      'Europe/Ljubljana',
      'Europe/London',
      'Europe/Luxembourg',
      'Europe/Madrid',
      'Europe/Malta',
      'Europe/Mariehamn',
      'Europe/Minsk',
      'Europe/Monaco',
      'Europe/Moscow',
      'Europe/Oslo',
      'Europe/Paris',
      'Europe/Podgorica',
      'Europe/Prague',
      'Europe/Riga',
      'Europe/Rome',
      'Europe/Samara',
      'Europe/San_Marino',
      'Europe/Sarajevo',
      'Europe/Saratov',
      'Europe/Simferopol',
      'Europe/Skopje',
      'Europe/Sofia',
      'Europe/Stockholm',
      'Europe/Tallinn',
      'Europe/Tirane',
      'Europe/Ulyanovsk',
      'Europe/Uzhgorod',
      'Europe/Vaduz',
      'Europe/Vatican',
      'Europe/Vienna',
      'Europe/Vilnius',
      'Europe/Volgograd',
      'Europe/Warsaw',
      'Europe/Zagreb',
      'Europe/Zaporozhye',
      'Europe/Zurich',
      'Indian/Antananarivo',
      'Indian/Chagos',
      'Indian/Christmas',
      'Indian/Cocos',
      'Indian/Comoro',
      'Indian/Kerguelen',
      'Indian/Mahe',
      'Indian/Maldives',
      'Indian/Mauritius',
      'Indian/Mayotte',
      'Indian/Reunion',
      'Pacific/Apia',
      'Pacific/Auckland',
      'Pacific/Bougainville',
      'Pacific/Chatham',
      'Pacific/Chuuk',
      'Pacific/Easter',
      'Pacific/Efate',
      'Pacific/Enderbury',
      'Pacific/Fakaofo',
      'Pacific/Fiji',
      'Pacific/Funafuti',
      'Pacific/Galapagos',
      'Pacific/Gambier',
      'Pacific/Guadalcanal',
      'Pacific/Guam',
      'Pacific/Honolulu',
      'Pacific/Kiritimati',
      'Pacific/Kosrae',
      'Pacific/Kwajalein',
      'Pacific/Majuro',
      'Pacific/Marquesas',
      'Pacific/Midway',
      'Pacific/Nauru',
      'Pacific/Niue',
      'Pacific/Norfolk',
      'Pacific/Noumea',
      'Pacific/Pago_Pago',
      'Pacific/Palau',
      'Pacific/Pitcairn',
      'Pacific/Pohnpei',
      'Pacific/Port_Moresby',
      'Pacific/Rarotonga',
      'Pacific/Saipan',
      'Pacific/Tahiti',
      'Pacific/Tarawa',
      'Pacific/Tongatapu',
      'Pacific/Wake',
      'Pacific/Wallis',
    ];
  }
});

async function saveProfile() {
  if (!user) {
    console.error('No user available');
    return;
  }

  isSaving.value = true;
  try {
    const payload = {
      ...user,
      description: userDescription.value,
      timezone: userTimezone.value,
    };

    await apiClient.updateUser(payload);

    // Update window user object
    if (window.CROW_USER) {
      window.CROW_USER.description = userDescription.value;
      window.CROW_USER.timezone = userTimezone.value;
    }

    notifications.notify({
      title: t('user.settings.general.profile_saved'),
      type: 'success',
    });
  } catch (error) {
    console.error('Failed to save profile:', error);
    notifications.notify({
      title: t('user.settings.general.profile_save_error'),
      type: 'error',
    });
  } finally {
    isSaving.value = false;
  }
}

const localeOptions = computed(() =>
  SUPPORTED_LOCALES.map((supportedLocale) => ({
    value: supportedLocale,
    text: new Intl.DisplayNames(supportedLocale, { type: 'language' }).of(supportedLocale) || supportedLocale,
  })),
);

const storedLocale = useStorage('crow:locale', locale.value);
const selectedLocale = computed<string>({
  async set(_selectedLocale) {
    await setI18nLanguage(_selectedLocale);
    storedLocale.value = _selectedLocale;
  },
  get() {
    return storedLocale.value;
  },
});

// Pipeline split ratio input
const storedSplitRatio = useStorage('crow:pipeline-sidebar-size', 30);
const pipelineSplitRatioInput = computed<number>({
  set(value: number) {
    // Clamp value between 25 and 50
    const clampedValue = Math.max(25, Math.min(50, value));
    storedSplitRatio.value = clampedValue;
  },
  get() {
    return storedSplitRatio.value;
  },
});

// Log theme options
const logThemeOptions = computed(() => [
  { value: 'auto', text: t('user.settings.general.log_theme_system') },
  { value: 'light', text: t('user.settings.general.log_theme_light') },
  { value: 'dark', text: t('user.settings.general.log_theme_dark') },
]);

const logTheme = useStorage('crow:log-theme', 'auto');

const autoFollowEnabled = useStorage('crow:pipeline-auto-follow', true);

useCrowTitle(computed(() => [t('user.settings.general.general'), t('user.settings.settings')]));
</script>
