# راهنمای انتشار نسخه اختصاصی 3x-ui

نسخه این بسته: `v3.5.3`

## تغییرات اصلی

- حذف لینک تلگرام XrayUI از داشبورد
- حذف دکمه‌های مستندات و حمایت مالی Sanaei از نوار کناری
- افزودن ری‌استارت زمان‌بندی‌شده در تنظیمات Xray > پایه
- انتخاب بازه برحسب دقیقه، ساعت یا روز
- ری‌استارت فقط Xray در حالت عادی و ری‌استارت کامل x-ui با گزینه اختیاری

## آدرس نصب آخرین نسخه پایدار

```bash
bash <(curl -Ls https://raw.githubusercontent.com/0fariid0/3x-ui/main/install.sh)
```

## نصب دقیق نسخه 3.5.3

```bash
bash <(curl -Ls https://raw.githubusercontent.com/0fariid0/3x-ui/main/install.sh) v3.5.3
```

## انتشار

پس از Commit و Push فایل‌ها، تگ زیر را Push کنید:

```bash
git tag -a v3.5.3 -m "v3.5.3 - scheduled Xray and panel restarts"
git push origin v3.5.3
```

Workflow بخش Actions فایل‌های Linux و Windows را ساخته و به Release همان تگ اضافه می‌کند.
