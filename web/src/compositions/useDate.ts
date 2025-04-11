import { useI18n } from 'vue-i18n';

let currentLocale = 'en';

function splitDuration(durationMs: number) {
  const totalSeconds = durationMs / 1000;
  const totalMinutes = totalSeconds / 60;
  const totalHours = totalMinutes / 60;

  const seconds = Math.floor(totalSeconds) % 60;
  const minutes = Math.floor(totalMinutes) % 60;
  const hours = Math.floor(totalHours) % 24;

  return {
    seconds,
    minutes,
    hours,
    totalHours,
    totalMinutes,
    totalSeconds,
  };
}

function toLocaleString(date: Date) {
  return date.toLocaleString(currentLocale, {
    dateStyle: 'short',
    timeStyle: 'short',
  });
}

function timeAgo(date: number) {
  const seconds = Math.floor((new Date().getTime() - date) / 1000);

  const formatter = new Intl.RelativeTimeFormat(currentLocale);

  let interval = seconds / 31536000;
  if (interval > 1) {
    return formatter.format(-Math.round(interval), 'year');
  }
  interval = seconds / 2592000;
  if (interval > 1) {
    return formatter.format(-Math.round(interval), 'month');
  }
  interval = seconds / 86400;
  if (interval > 1) {
    return formatter.format(-Math.round(interval), 'day');
  }
  interval = seconds / 3600;
  if (interval > 1) {
    return formatter.format(-Math.round(interval), 'hour');
  }
  interval = seconds / 60;
  if (interval > 0.5) {
    return formatter.format(-Math.round(interval), 'minute');
  }
  return useI18n().t('time.just_now');
}

function prettyDuration(durationMs: number) {
  const t = splitDuration(durationMs);

  if (t.totalHours > 1) {
    const hours = Math.floor(t.totalHours);
    const totalMinutesRemaining = (t.totalHours - hours) * 60;
    const minutes = Math.floor(totalMinutesRemaining);
    const seconds = Math.round((totalMinutesRemaining - minutes) * 60);
    return `${hours}h ${minutes}m ${seconds}s`;
  }
  if (t.totalMinutes > 1) {
    const minutes = Math.floor(t.totalMinutes);
    const seconds = Math.round((t.totalMinutes - minutes) * 60);
    const remainingSeconds = Math.round(seconds % 60);
    const formattedMinutes =
      minutes > 0
        ? Intl.NumberFormat(currentLocale, { style: 'unit', unit: 'minute', unitDisplay: 'short' }).format(minutes)
        : '';

    const formattedSeconds =
      remainingSeconds > 0 || minutes === 0
        ? Intl.NumberFormat(currentLocale, { style: 'unit', unit: 'second', unitDisplay: 'short' }).format(
            remainingSeconds,
          )
        : '';

    return minutes > 0 && remainingSeconds > 0
      ? `${formattedMinutes} ${formattedSeconds}`
      : `${formattedMinutes}${formattedSeconds}`;
  }
  return Intl.NumberFormat(currentLocale, { style: 'unit', unit: 'second', unitDisplay: 'short' }).format(
    Math.round(t.totalSeconds),
  );
}

function durationAsNumber(durationMs: number): string {
  const { seconds, minutes, hours } = splitDuration(durationMs);

  const minSecFormat = `${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;

  if (hours > 0) {
    return `${hours.toString().padStart(2, '0')}:${minSecFormat}`;
  }

  return minSecFormat;
}

export function useDate() {
  async function setDateLocale(locale: string) {
    currentLocale = locale;
  }

  return {
    toLocaleString,
    timeAgo,
    prettyDuration,
    setDateLocale,
    durationAsNumber,
  };
}
