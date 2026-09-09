// ignore: unused_import
import 'package:intl/intl.dart' as intl;
import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for Russian (`ru`).
class AppLocalizationsRu extends AppLocalizations {
  AppLocalizationsRu([String locale = 'ru']) : super(locale);

  @override
  String get appName => 'ClubPay';

  @override
  String get welcome => 'Твоё время.\nТвоя игра.';

  @override
  String get intro =>
      'Войдите, чтобы сохранять игровое время и возвращаться к игре в любимых клубах.';

  @override
  String get phone => 'Номер телефона';

  @override
  String get phoneHint => '+998 90 123 45 67';

  @override
  String get phoneError => 'Введите номер Узбекистана: +998 и 9 цифр';

  @override
  String get continueLabel => 'Продолжить';

  @override
  String get telegramTitle => 'Подтвердите номер';

  @override
  String get telegramHelp =>
      'Откройте бота ClubPay, отправьте свой контакт и введите полученный код здесь. Код действует не более 3 минут.';

  @override
  String get openTelegram => 'Открыть Telegram';

  @override
  String get otp => 'Код из Telegram';

  @override
  String get verify => 'Войти';

  @override
  String get newChallenge => 'Получить новый код';

  @override
  String get challengeExpired => 'Запрос устарел. Получите новый код.';

  @override
  String get invalidOtp =>
      'Код неверный или устарел. Проверьте его или получите новый.';

  @override
  String get rateLimited => 'Слишком много попыток. Попробуйте через 15 минут.';

  @override
  String get serviceUnavailable =>
      'Вход через Telegram сейчас недоступен. Попробуйте позже.';

  @override
  String get networkError =>
      'Не удалось связаться с ClubPay. Проверьте подключение и повторите.';

  @override
  String get storageError =>
      'Не удалось сохранить сессию. Разрешите хранение данных и повторите вход.';

  @override
  String get home => 'Главная';

  @override
  String get profile => 'Профиль';

  @override
  String get scan => 'Сканировать QR';

  @override
  String get gameTime => 'Игровое время';

  @override
  String get perClub => 'Время хранится отдельно в каждом клубе.';

  @override
  String get emptyBalance => 'У вас пока нет игрового времени';

  @override
  String get emptyBalanceHelp =>
      'Отсканируйте QR на компьютере клуба. Неиспользованное время профильной сессии сохранится здесь.';

  @override
  String get chooseClub => 'Выберите клуб';

  @override
  String get unknownClub => 'Клуб ещё не выбран';

  @override
  String get startBalance => 'Начать с баланса';

  @override
  String get buyTime => 'Купить игровое время';

  @override
  String get balanceHelp =>
      'Выберите компьютер по QR, чтобы использовать время этого клуба.';

  @override
  String get scanHelp => 'Наведите камеру на QR ClubPay на компьютере.';

  @override
  String get qrInput => 'Ссылка или код QR';

  @override
  String get checkQr => 'Проверить QR';

  @override
  String get uploadQr => 'Загрузить изображение QR';

  @override
  String get camera => 'Открыть камеру';

  @override
  String get cameraHelp =>
      'Разрешите доступ к камере. Если камера недоступна, вставьте ссылку или загрузите QR.';

  @override
  String get invalidQr =>
      'QR не распознан. Используйте QR ClubPay с экрана компьютера.';

  @override
  String get qrNotFound =>
      'Этот QR не найден или больше недоступен. Проверьте ссылку либо уточните код у администратора клуба.';

  @override
  String get close => 'Закрыть';

  @override
  String get pc => 'Компьютер';

  @override
  String get zone => 'Зона';

  @override
  String get available => 'Доступен';

  @override
  String get sleeping => 'В режиме сна';

  @override
  String get occupied => 'Занят';

  @override
  String get maintenance => 'Недоступен';

  @override
  String get packages => 'Выберите игровое время';

  @override
  String get customAmount => 'Своя сумма, сум';

  @override
  String get customHelp => 'Сумма оплаты за игровое время на этом ПК.';

  @override
  String get providers => 'Способ оплаты';

  @override
  String get payme => 'Payme';

  @override
  String get click => 'Click';

  @override
  String get mock => 'Тестовая оплата';

  @override
  String get testPayAndStart => 'Тестово оплатить и запустить';

  @override
  String get noProviders => 'Оплата сейчас недоступна';

  @override
  String get pay => 'Оплатить';

  @override
  String get clubBalance => 'Ваше время в этом клубе';

  @override
  String startTime(String time) {
    return 'Начать на $time';
  }

  @override
  String duration(int hours, int minutes, int seconds) {
    return '$hours ч $minutes мин $seconds с';
  }

  @override
  String price(int amount) {
    return '$amount сум';
  }

  @override
  String get payment => 'Оплата и сессия';

  @override
  String get waitingPayment => 'Ожидаем оплату';

  @override
  String get startingSession => 'Запускаем сессию';

  @override
  String get startingHelp =>
      'Оплата подтверждена. Ожидаем подтверждение запуска компьютером клуба. Это может занять некоторое время.';

  @override
  String get sessionReady => 'Сессия запущена';

  @override
  String get sessionEnded => 'Сессия завершена';

  @override
  String get paymentFailed => 'Оплата не завершена';

  @override
  String get startFailed => 'Не удалось подтвердить запуск';

  @override
  String get checkStatus => 'Повторить проверку статуса';

  @override
  String get openPayment => 'Открыть страницу оплаты';

  @override
  String get returnHelp =>
      'После оплаты вернитесь в ClubPay. Статус обновится автоматически.';

  @override
  String get operationPending => 'Проверяем результат операции';

  @override
  String get operationHelp =>
      'Не создавайте повторную оплату. Проверьте статус ещё раз.';

  @override
  String get language => 'Язык';

  @override
  String get russian => 'Русский';

  @override
  String get uzbek => 'O‘zbekcha';

  @override
  String get logout => 'Выйти из аккаунта';

  @override
  String get name => 'Имя';

  @override
  String get refresh => 'Обновить';

  @override
  String get back => 'Назад';

  @override
  String get loading => 'Загружаем ClubPay';

  @override
  String get resumePayment => 'Продолжить проверку сессии';

  @override
  String get noName => 'Имя не указано';

  @override
  String get scanToBuy =>
      'Отсканируйте QR компьютера, чтобы выбрать время и оплатить.';

  @override
  String get noTime => 'Нет сохранённого времени';

  @override
  String get imageTooLarge => 'Выберите PNG или JPEG до 5 МБ.';

  @override
  String get amountError => 'Введите целую сумму больше нуля.';

  @override
  String get checkSlow =>
      'Подтверждение занимает больше времени. Повторите проверку статуса.';

  @override
  String get sessionExpired => 'Сессия входа завершена. Войдите снова.';

  @override
  String get paymentUnavailable =>
      'Операция не выполнена. Обновите данные компьютера.';

  @override
  String get authRetry => 'Повторить вход';

  @override
  String get signInHelp =>
      'Подтверждение номера через Telegram. Ваши остатки времени будут доступны после входа.';

  @override
  String get readyToPlay => 'Готовы играть?';

  @override
  String get welcomeBack => 'С возвращением';

  @override
  String get homeSubtitle => 'Ваш компьютер. Ваше время. В один скан.';

  @override
  String get accountSubtitle => 'Ваш аккаунт и настройки приложения.';

  @override
  String get yourAccount => 'Ваш аккаунт';

  @override
  String get aboutApp => 'О приложении';

  @override
  String get startWithQr => 'Начните с QR на компьютере';

  @override
  String get tapQrBelow =>
      'Нажмите кнопку QR внизу и наведите камеру на QR свободного ПК.';

  @override
  String get timeReturnsHere =>
      'Отсканируйте QR в клубе. После игры здесь появится клуб и сохранённый остаток.';

  @override
  String get howItWorks => 'Как это работает';

  @override
  String get guideScan => 'Сканируйте QR на ПК';

  @override
  String get guideScanHelp =>
      'Выберите свободный компьютер в клубе и нажмите кнопку QR внизу приложения.';

  @override
  String get guideChoose => 'Выберите игровое время';

  @override
  String get guideChooseHelp =>
      'Купите время или используйте остаток этого клуба. В другой зоне время пересчитается по её цене.';

  @override
  String get guidePlay => 'Играйте. Остаток сохранится';

  @override
  String get guidePlayHelp =>
      'Дождитесь подтверждения запуска. Если закончите раньше, оставшееся время вернётся в профиль.';

  @override
  String get gotIt => 'Понятно';

  @override
  String get cameraUnavailable => 'Камера недоступна';

  @override
  String get cameraFallbackHelp =>
      'Разрешите доступ к камере в настройках приложения или браузера, затем попробуйте ещё раз.';

  @override
  String get retryCamera => 'Попробовать снова';

  @override
  String get checkingQr => 'Проверяем компьютер…';

  @override
  String get scanAgain => 'Сканировать ещё раз';

  @override
  String get pointCamera => 'Наведите камеру на QR компьютера';

  @override
  String get scanAutomatic => 'Он распознается автоматически.';

  @override
  String get qrPhoto => 'Фото QR';

  @override
  String get manualQr => 'Ввести код';

  @override
  String get manualQrHelp =>
      'Вставьте ссылку или код из QR компьютера ClubPay.';

  @override
  String get cameraStarting => 'Подключаем камеру';

  @override
  String get cameraPermissionHelp =>
      'Разрешите доступ к камере, когда появится запрос.';

  @override
  String get qrCatalogUnavailable =>
      'Не удалось проверить QR: сервер клуба сейчас недоступен. Код не обязательно устарел. Повторите проверку позже.';

  @override
  String get retryQr => 'Повторить проверку';

  @override
  String get liveCatalogPreview => 'Рабочий ПК · просмотр';

  @override
  String get liveCatalogPreviewHelp =>
      'Данные компьютера получены из рабочего клуба. Вы вошли в тестовый профиль: здесь можно посмотреть ПК и тарифы, но реальные оплаты и остатки времени недоступны.';

  @override
  String get myClubs => 'Мои клубы';

  @override
  String get yourTime => 'Ваше время';

  @override
  String get zoneConversionHelp =>
      'Один остаток на клуб. Доступное время пересчитывается по цене выбранной зоны.';

  @override
  String get clubOffline => 'Клуб сейчас не на связи';

  @override
  String get balanceStale => 'Нет связи. Показан последний полученный остаток.';

  @override
  String balanceUpdated(String time) {
    return 'Обновлено: $time';
  }

  @override
  String get balancePending => 'Обновляем остаток…';

  @override
  String get useBalance => 'Использовать время';

  @override
  String balanceInZone(String zone) {
    return 'Доступно в зоне $zone';
  }

  @override
  String get balanceUnknown =>
      'Не удалось получить остаток. Попробуйте обновить.';
}
