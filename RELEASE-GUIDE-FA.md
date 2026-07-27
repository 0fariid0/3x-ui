# راهنمای انتشار نسخه اختصاصی 3x-ui

نسخه این بسته: `v3.5.5`

## تغییرات اصلی

- مانیتور سلامت Xray و ری‌استارت خودکار فقط پس از تشخیص خرابی واقعی
- تأیید چند بررسی ناموفق، فاصله امن و مدار محافظ برای جلوگیری از حلقه ری‌استارت
- نادیده‌گرفتن توقف دستی و بازه ری‌استارت عمدی در تشخیص خرابی
- اجرای ری‌استارت زمان‌بندی‌شده روی مرزهای واقعی ساعت؛ مانند :۰۰، :۱۵، :۳۰ و :۴۵
- کپی لینک با کلیک روی QR به‌جای دانلود خودکار تصویر
- افزودن `?name=email` به لینک‌های Subscription و نام‌گذاری کانفیگ‌ها با ایمیل کاربر
- حذف پورت از انتهای آدرس‌های Host؛ پورت مشترک Host روی همه آدرس‌ها و مقدار ۰ برابر پورت Inbound
- هدر Host جداگانه برای هر IP یا دامنه
- هماهنگ‌سازی تست‌های Host، Subscription، زمان‌بندی و فایل‌های تولیدشده OpenAPI

## آدرس نصب آخرین نسخه پایدار

```bash
bash <(curl -Ls https://raw.githubusercontent.com/0fariid0/3x-ui/main/install.sh)
```

## نصب یا آپدیت دقیق نسخه 3.5.5

```bash
bash <(curl -Ls https://raw.githubusercontent.com/0fariid0/3x-ui/main/install.sh) v3.5.5
```

## انتشار

پس از Commit و Push فایل‌ها، تگ زیر را Push کنید:

```bash
git tag -a v3.5.5 -m "v3.5.5 - actual client names and resilient updater downloads"
git push origin v3.5.5
```

Workflow بخش Actions فایل‌های Linux و Windows را ساخته و به Release همان تگ اضافه می‌کند.
