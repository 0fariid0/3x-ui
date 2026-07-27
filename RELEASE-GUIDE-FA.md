# راهنمای انتشار نسخه اختصاصی 3x-ui

نسخه این بسته: `v3.5.6`

## تغییرات اصلی

- ثبت زمان آخرین دریافت لینک Subscription برای هر کلاینت
- تشخیص نام و نسخه برنامه‌ای که Subscription را دریافت کرده است
- نگهداری حداکثر سه برنامه متفاوت و اخیراً استفاده‌شده
- نمایش نوع خروجی دریافت‌شده: RAW، JSON یا Clash
- نمایش تعداد دفعات استفاده هر برنامه
- نمایش همه خروجی‌های مدیریت‌شده هر کلاینت در تب «لینک‌ها» همراه نام و آدرس
- امکان غیرفعال‌کردن هر Host یا خروجی پیش‌فرض فقط برای همان کلاینت
- اعمال غیرفعال‌سازی روی RAW، JSON، Clash و لینک‌های نمایش‌داده‌شده داخل پنل

## آدرس نصب آخرین نسخه پایدار

```bash
bash <(curl -Ls https://raw.githubusercontent.com/0fariid0/3x-ui/main/install.sh)
```

## نصب یا آپدیت دقیق نسخه 3.5.6

```bash
bash <(curl -Ls https://raw.githubusercontent.com/0fariid0/3x-ui/main/install.sh) v3.5.6
```

## انتشار

پس از Commit و Push سورس، تگ زیر را Push کنید:

```bash
git tag -a v3.5.6 -m "v3.5.6 - subscription activity and per-client link visibility"
git push origin v3.5.6
```

Workflow بخش Actions فایل‌های Linux و Windows را ساخته و به Release همان تگ اضافه می‌کند.
