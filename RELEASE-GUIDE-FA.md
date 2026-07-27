# راهنمای انتشار نسخه اختصاصی 3x-ui

نسخه این بسته: `v3.5.2`

## آدرس نصب آخرین نسخه پایدار

```bash
bash <(curl -Ls https://raw.githubusercontent.com/0fariid0/3x-ui/main/install.sh)
```

## نصب دقیق نسخه 3.5.2

```bash
bash <(curl -Ls https://raw.githubusercontent.com/0fariid0/3x-ui/main/install.sh) v3.5.2
```

## انتشار

پس از Commit و Push فایل‌ها، تگ زیر را Push کنید:

```bash
git tag -a v3.5.2 -m "v3.5.2 - exact per-host templates and WARP routing"
git push origin v3.5.2
```

Workflow بخش Actions فایل‌های Linux و Windows را ساخته و به Release همان تگ اضافه می‌کند.
