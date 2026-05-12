***

# MihaniRelay 🚀

**رابط کاربری گرافیکی قدرتمند برای MihaniRelay (نسخه Go)**





رابط کاربری مدرن و سریع با پشتیبانی کامل از Google Apps Script، Cloudflare Worker، Xray/V2Ray و GitHub Codespace.

***

## ✨ ویژگی‌های کلیدی

- 🌐 پروکسی HTTP + SOCKS5 داخلی (سازگار با مرورگر، تلگرام، Discord و برنامه‌های دیگر)
- 🔄 Domain Fronting با Google Apps Script و Cloudflare Worker (پنهان‌سازی حداکثری ترافیک)
- ⚡ پشتیبانی از چندین Deployment ID (تا ۲۰ رله همزمان + لود بالانسینگ هوشمند)
- 📊 پینگ بلادرنگ و نمایش وضعیت سلامت رله‌ها
- 🔐 نصب خودکار گواهی CA (MITM شفاف برای ترافیک HTTPS)
- 🚀 ادغام کامل با Xray/V2Ray (پروتکل‌های VMess، VLESS، Trojan، Shadowsocks، Reality، NaïveProxy)
- ☁️ پشتیبانی از Cloudflare Worker به‌عنوان رله جایگزین یا مکمل
- 🐙 ادغام GitHub Codespace (ساخت، اجرا و مدیریت مستقیم پروژه از داخل GitHub)
- 🎨 تم و ظاهر قابل تنظیم (Dark/Light + تم‌های سفارشی)
- 🌍 پشتیبانی از سه زبان (فارسی -  English -  Finglish)
- 📝 سطوح مختلف لاگ‌گیری (Debug / Info / Warning / Error / Silent)
- 🛡️ حالت Exit Node (دور زدن Cloudflare Protection با تونلینگ هوشمند)
- 💾 ذخیره‌سازی امن تنظیمات + امکان Import/Export پروفایل‌ها

***

## 🏗️ معماری جدید (Go)

برنامه به‌طور کامل با زبان **Go** بازنویسی شده تا سرعت، پایداری و مصرف منابع بهینه شود. [ossinsight](https://ossinsight.io/analyze/masterking32/MasterHttpRelayVPN)

معماری کلی:

```text
Browser / Telegram / Apps
           │
   HTTP Proxy (8085)
   SOCKS5 Proxy (1080)
           │
           ▼
        MHRV-GO Core
           │
 ┌─────────┼───────────────┐
 │         │               │
 ▼         ▼               ▼
Google Fronting   Cloudflare Worker    Xray Core
   │                   │               │
Apps Script        CF Worker Script   V2Ray Protocols
```

***

## 📦 پیش‌نیازها

- Windows 10/11 (با WebView2) -  Linux -  macOS
- Go 1.22+ (برای اجرای مستقیم از سورس)
- حساب Google (برای Google Apps Script – اختیاری ولی توصیه‌شده)
- حساب Cloudflare (برای Cloudflare Worker – اختیاری)
- حساب GitHub (برای استفاده از GitHub Codespace – اختیاری)

***

## 🚀 راه‌اندازی سریع

### ۱. رله Google Apps Script (اختیاری اما توصیه‌شده)

1. به آدرس [script.google.com](https://script.google.com) بروید.
2. یک پروژه جدید ایجاد کنید.
3. محتوای فایل `Code.gs` موجود در این مخزن را در ادیتور paste کنید.
4. مقدار `AUTH_KEY` را به یک رمز **قوی و یکتا** تغییر دهید.
5. از منوی Deploy گزینه **New deployment** را انتخاب کنید.
6. Type را روی **Web app** بگذارید، `Execute as: Me` و `Access: Anyone`.
7. روی Deploy کلیک کرده و Deployment ID را کپی کنید.

### ۲. رله Cloudflare Worker

1. وارد داشبورد Cloudflare شوید و به بخش **Workers** بروید.
2. یک Worker جدید بسازید.
3. محتوای فایل `worker.js` موجود در مخزن را در اسکریپت Worker قرار دهید.
4. در قسمت Variables یک Secret با نام `AUTH_KEY` تعریف کنید (مطابق با AUTH_KEY سمت کلاینت).
5. URL نهایی Worker را کپی کرده و در تنظیمات برنامه وارد کنید.

### ۳. نصب و اجرای برنامه

#### اجرای مستقیم از سورس

```bash
git clone https://github.com/yourusername/mhrv-go.git
cd mhrv-go
go run .
```

#### استفاده از فایل اجرایی آماده

1. به بخش **Releases** مخزن GitHub بروید.
2. فایل اجرایی متناسب با سیستم‌عامل خود را دانلود کنید.
3. فایل را اجرا کنید، برنامه GUI به‌صورت خودکار باز می‌شود.

***

## 🎛️ امکانات پیشرفته

### ادغام Xray / V2Ray

- افزودن کانفیگ‌های VMess، VLESS، Trojan، Shadowsocks، Reality و NaïveProxy
- تست خودکار پینگ و سلامت کانفیگ‌ها
- اتصال هوشمند (Fallback و Load Balancing بین چند رله)
- استفاده به‌عنوان Exit Node برای ترافیک غیر HTTP/HTTPS

### GitHub Codespace Mode

- ورود و احراز هویت با GitHub OAuth
- ساخت خودکار Codespace برای این مخزن
- اجرای پروژه داخل Codespace (بدون نیاز به تنظیمات لوکال)
- مدیریت رله‌ها و لاگ‌ها از طریق محیط وب GitHub

### حالت Exit Node

- ترافیکی که از لایه Cloudflare Protection عبور نمی‌کند، به‌صورت هوشمند به Exit Node (Xray) هدایت می‌شود.
- مناسب برای عبور از فیلترها و محدودیت‌های سخت روی برخی وب‌سایت‌ها و سرویس‌ها.

### Cloudflare Worker + Google Hybrid

- انتخاب خودکار بهترین مسیر بین Google Apps Script و Cloudflare Worker در لحظه.
- امکان تعریف چندین Deployment ID و Worker برای افزایش پایداری و سرعت.

### تنظیمات ظاهری و لاگ

- تغییر تم (Dark، Light و تم‌های سفارشی)
- انتخاب زبان (فارسی، انگلیسی، فینگلیش)
- تنظیم سطح لاگ‌گیری (Debug / Info / Warning / Error / Silent)
- تنظیم فونت و اندازه متن محیط کاربری

***

## 📱 نحوه استفاده از پروکسی

### مرورگرها

- HTTP Proxy: `127.0.0.1:8085`

پیشنهاد برای مدیریت پروفایل‌ها:

- افزونه **SwitchyOmega** برای Chrome/Edge
- افزونه **FoxyProxy** برای Firefox

### تلگرام و سایر برنامه‌ها

- SOCKS5 Proxy: `127.0.0.1:1080`

در تنظیمات برنامه مقصد، آدرس و پورت بالا را وارد کرده و در صورت نیاز، گزینه‌های DNS over proxy را فعال کنید.

***

## 🛠️ رفع مشکلات رایج

### جدول خطاها و راه‌حل‌ها

| مشکل                       | راه‌حل                                                                 |
|---------------------------|------------------------------------------------------------------------|
| 502 Bad JSON              | Deployment ID و مقدار `AUTH_KEY` را بررسی کنید.                      |
| Certificate Error         | گواهی CA را دوباره نصب کنید و مرورگر را به‌طور کامل ببندید و باز کنید. |
| سرعت پایین               | از چند Deployment ID و ترکیب Google + Cloudflare همزمان استفاده کنید. |
| Cloudflare Block          | حالت **Exit Node Mode** را فعال کنید.                                |
| پینگ بالا / ناپایداری    | از قابلیت Scan IP و Performance Mode استفاده کنید.                  |

***

## 📁 ساختار پروژه

```text
mhrv-go/
├── main.go
├── go.mod
├── go.sum
├── index.html
├── icon.ico
│
├── config/    # مدیریت تنظیمات و پروفایل‌ها
├── cert/      # گواهی‌های CA و TLS
├── mitm/      # ماژول‌های MITM برای HTTPS
├── fronter/   # منطق Domain Fronting (Google / CF)
├── proxy/     # هسته پروکسی HTTP/SOCKS5
├── gui/       # رابط کاربری گرافیکی (WebView)
├── ws/        # ارتباط WebSocket بین GUI و Core
```

***

## 📜 سلب مسئولیت

- این پروژه صرفاً برای اهداف آموزشی و تحقیقاتی توسعه داده شده است. [sourceforge](https://sourceforge.net/projects/masterhttprelayvpn.mirror/)
- توسعه‌دهندگان هیچ‌گونه مسئولیتی در قبال استفاده غیرقانونی، دور زدن قوانین محلی یا نقض Terms of Service سرویس‌دهنده‌ها ندارند. [github](https://github.com/masterking32/MasterHttpRelayVPN)
- استفاده از سرویس‌های Google، Cloudflare و GitHub باید کاملاً مطابق با قوانین و شرایط استفاده رسمی آن‌ها باشد. [sourceforge](https://sourceforge.net/projects/masterhttprelayvpn.mirror/)

***

## 🙏 تشکر و اعتبار

- پروژه اصلی: [`masterking32/MasterHttpRelayVPN`](https://github.com/masterking32/MasterHttpRelayVPN) [github](https://github.com/topics/masterhttprelayvpn)
- بازنویسی و توسعه GUI پیشرفته: **Faz Pad Studio**

اگر این پروژه برایت مفید بود، فراموش نکن یک ⭐ روی مخزن GitHub بگذاری.
