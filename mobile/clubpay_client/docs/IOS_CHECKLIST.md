# Этап Xcode / iPhone — ещё не проверен

iOS scaffold создан Flutter, но Xcode build, Simulator и физический iPhone
не запускались. Проверка plist-синтаксиса не является проверкой iOS.

1. Выбрать Development Team, provisioning, финальный Bundle ID и минимальную
   iOS-версию с учётом plugin requirements. Сборка Debug/Release на iPhone.
2. Проверить Swift Package Manager / CocoaPods совместимость подключённых
   plugins. Добавленные `Runner.entitlements` должны войти в signing.
3. Keychain: запись/чтение access+refresh, холодный старт, ротация, logout,
   переустановка, обновление, блокировка устройства, отказ записи и offline.
4. Камера: первый запрос RU/UZ разрешения, отказ, повторное разрешение через
   Settings, autofocus, низкий свет, QR на мониторе, повторное распознавание,
   сворачивание и освобождение камеры, landscape. `NSCameraUsageDescription`
   и локализованные InfoPlist.strings подготовлены.
5. Выбор QR PNG/JPEG через Files/Photos, ограничения размера, повреждённые
   изображения, разрешения и отмена выбора.
6. Telegram: открытие системным браузером/приложением, подтверждение именно
   своего контакта, несовпадение телефона, получение и ввод OTP, TTL, rate limits.
7. Universal Links: выбрать контролируемый HTTPS origin для
   `MOBILE_RETURN_BASE_URL`, добавить `applinks:<domain>` в Associated Domains,
   разместить `apple-app-site-association` с реальными Team ID/Bundle ID.
   Поддержать `/payment/return` и `/qr/*`; проверить cold/warm start,
   сохранение deep link через splash/login и возвращение после OTP.
   Домены и AASA намеренно не выдуманы и пока не опубликованы.
8. Click и Payme: переход в Safari/установленное приложение провайдера,
   успешный платёж, отмена, задержка callback, возврат по Universal Link,
   ручное возвращение, повторный возврат, истёкший access token, закрытие
   приложения во время оплаты, потеря сети. Никаких WebView экранов ClubPay.
9. Реальный клуб: Cloud → Controller → Agent, pending grant, delayed/offline
   Controller, восстановление подключения, подтверждение accepted, остаток
   секунд после завершения и повторный запуск с него без двойного списания.
10. Safe areas, клавиатура и OTP autofill, Dynamic Type, VoiceOver, RU/UZ,
    зоны нажатия, нижняя навигация, фон/передний план, отсутствие дублей polling.
11. Финальные иконки приложения и launch assets, privacy manifest, сведения
    о собираемых данных и требования App Store — отдельный этап перед выпуском.
