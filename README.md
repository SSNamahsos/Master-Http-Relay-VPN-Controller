# MihaniRelay 🚀
**رابط کاربری گرافیکی قدرتمند برای MihaniRelay (نسخه Go)**

![GitHub License](https://img.shields.io/github/license/yourusername/mhrv-go)
![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-blue)
![Language](https://img.shields.io/badge/Language-Go%20%2B%20WebView-brightgreen)

**رابط کاربری مدرن و سریع با پشتیبانی کامل از Google Apps Script، Cloudflare Worker، Xray/V2Ray و GitHub Codespace.**

---

## ✨ ویژگی‌های کلیدی

- **🌐 پروکسی HTTP + SOCKS5 داخلی** (پشتیبانی کامل از مرورگر، تلگرام، Discord و ...)
- **🔄 Domain Fronting** با Google + Cloudflare Worker (ترافیک کاملاً پنهان)
- **⚡ پشتیبانی از چندین Deployment ID** (تا ۲۰ رله همزمان + لود بالانسینگ هوشمند)
- **📊 پینگ بلادرنگ** و نمایش وضعیت سلامت رله‌ها
- **🔐 نصب خودکار گواهی CA** (MITM شفاف برای HTTPS)
- **🚀 ادغام کامل Xray/V2Ray** (Vmess, Vless, Trojan, Shadowsocks, Reality, NaïveProxy)
- **☁️ پشتیبانی از Cloudflare Worker** به عنوان رله جایگزین/مکمل
- **🐙 GitHub Codespace Integration** (ساخت، اجرا و مدیریت مستقیم از داخل GitHub)
- **🎨 تم و ظاهر قابل تغییر** (Dark/Light + تم‌های سفارشی)
- **🌍 پشتیبانی از سه زبان** (فارسی • English • Finglish)
- **📝 سطوح مختلف لاگ‌گیری** (Debug / Info / Warning / Error / Silent)
- **🛡️ Exit Node Mode** (دور زدن حفاظت Cloudflare با تونلینگ هوشمند)
- **💾 ذخیره‌سازی امن تنظیمات** + Import/Export

---

## 🏗️ معماری جدید (Go)

برنامه به‌طور کامل با زبان **Go** بازنویسی شده تا سرعت، پایداری و مصرف منابع بهینه شود.
Browser / Telegram
│
HTTP Proxy (8085) ─────┐
SOCKS5 Proxy (1080) ───┤
▼
MHRV-GO Core
│
┌──────────────┼────────────────────┐
│              │                    │
Google Fronting   Cloudflare Worker    Xray Core
│              │                    │
Apps Script       CF Worker Script     V2Ray Protocols
text---

## 📦 پیش‌نیازها

- **Windows 10/11** (WebView2) | **Linux** | **macOS**
- Go 1.22+
- حساب Google (اختیاری)
- حساب Cloudflare (اختیاری)
- حساب GitHub (برای Codespace)

---

## 🚀 راه‌اندازی سریع

### ۱. رله Google Apps Script (اختیاری اما توصیه‌شده)
1. به [script.google.com](https://script.google.com) بروید.
2. پروژه جدید بسازید و محتوای `Code.gs` را paste کنید.
3. `AUTH_KEY` را به رمز قوی تغییر دهید.
4. Deploy → New deployment → Web app → Execute as: Me → Access: Anyone
5. Deployment ID را کپی کنید.

### ۲. رله Cloudflare Worker (جدید)
1. به داشبورد Cloudflare Workers بروید.
2. Worker جدید بسازید و اسکریپت موجود در مخزن (`worker.js`) را قرار دهید.
3. Secret `AUTH_KEY` را تعریف کنید.
4. URL Worker را در برنامه وارد کنید.

### ۳. اجرای برنامه

```bash
git clone https://github.com/yourusername/mhrv-go.git
cd mhrv-go
go run .
یا فایل اجرایی آماده را از Releases دانلود کنید.

🎛️ امکانات جدید
ادغام Xray / V2Ray

افزودن کانفیگ‌های VMess, VLESS, Trojan, Shadowsocks, Reality
تست پینگ خودکار کانفیگ‌ها
اتصال هوشمند (Fallback)
استفاده به‌عنوان Exit Node برای ترافیک غیر-HTTP
پشتیبانی از NaïveProxy

GitHub Codespace Mode

ورود با GitHub OAuth
ساخت خودکار Codespace
اجرای پروژه داخل Codespace
مدیریت رله‌ها از طریق وب

حالت Exit Node
ترافیک‌هایی که از Cloudflare Protection رد نمی‌شوند، به‌طور هوشمند به Exit Node (Xray) هدایت می‌شوند.
Cloudflare Worker + Google Hybrid
برنامه بهترین مسیر را بین Google Apps Script و Cloudflare Worker به‌صورت real-time انتخاب می‌کند.
تنظیمات ظاهری و لاگ

تغییر تم (Dark/Light/Custom)
انتخاب زبان (فارسی، انگلیسی، فینگلیش)
سطوح لاگ‌گیری قابل تنظیم
فونت و اندازه متن قابل تغییر


📱 نحوه استفاده
مرورگر:

HTTP Proxy: 127.0.0.1:8085

تلگرام و برنامه‌ها:

SOCKS5: 127.0.0.1:1080

افزونه‌های پیشنهادی:

SwitchyOmega (Chrome/Edge)
FoxyProxy (Firefox)


🛠️ رفع مشکل رایج





























مشکلراه‌حل502 Bad JSONبررسی Deployment ID و AUTH_KEYCertificate Errorنصب مجدد CA + بستن کامل مرورگرسرعت پاییناستفاده همزمان Google + Cloudflare + چندین IDCloudflare Blockفعال کردن Exit Node Modeپینگ بالاScan IP + Performance Mode

📁 ساختار پروژه
textmhrv-go/
├── main.go
├── go.mod
├── go.sum
├── index.html
├── icon.ico
│
├── config/
├── cert/
├── mitm/
├── fronter/
├── proxy/
├── gui/
├── ws/

📜 سلب مسئولیت
این پروژه صرفاً برای اهداف آموزشی و تحقیقاتی توسعه یافته است.
توسعه‌دهندگان هیچ مسئولیتی در قبال استفاده غیرقانونی یا نقض قوانین محلی ندارند. استفاده از سرویس‌های Google و Cloudflare باید مطابق با Terms of Service آنها باشد.

🙏 تشکر و اعتبار

پروژه اصلی: masterking32/MasterHttpRelayVPN
بازنویسی و توسعه GUI پیشرفته: Faz Pad Studio

ستاره ⭐ بده اگر مفید بود!
