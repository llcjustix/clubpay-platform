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
  String get telegramTitle => 'Введите код из SMS';

  @override
  String get telegramHelp =>
      'Мы отправили шестизначный код на ваш номер. Введите его в течение 3 минут.';

  @override
  String get openTelegram => 'Отправить SMS повторно';

  @override
  String get otp => 'Код из SMS';

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
      'Вход по SMS сейчас недоступен. Попробуйте позже.';

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
  String get emptyBalance => 'Игрового баланса пока нет';

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
  String tariffMinutes(int minutes) {
    String _temp0 = intl.Intl.pluralLogic(
      minutes,
      locale: localeName,
      other: '$minutes минуты',
      many: '$minutes минут',
      few: '$minutes минуты',
      one: '1 минута',
    );
    return '$_temp0';
  }

  @override
  String tariffHours(int hours) {
    String _temp0 = intl.Intl.pluralLogic(
      hours,
      locale: localeName,
      other: '$hours часа',
      many: '$hours часов',
      few: '$hours часа',
      one: '1 час',
    );
    return '$_temp0';
  }

  @override
  String price(int amount) {
    return '$amount сум';
  }

  @override
  String zoneHourlyPrice(int amount) {
    return '$amount сум/ч';
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
      'Мы отправим одно SMS с кодом. Ваши остатки времени будут доступны после входа.';

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
      'Отсканируйте QR компьютера в клубе. После игры здесь появится сохранённый баланс.';

  @override
  String get howItWorks => 'Как это работает';

  @override
  String get guideScan => 'Выберите клуб и компьютер';

  @override
  String get guideScanHelp =>
      'Найдите клуб, выберите зону и свободный компьютер.';

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
  String get clubGameBalance => 'Игровой баланс';

  @override
  String clubGameBalanceWithAmount(String amount) {
    return 'Игровой баланс: $amount';
  }

  @override
  String get currencySuffix => 'сум';

  @override
  String get clubBalanceHelp =>
      'Используйте его для игрового времени в этом клубе';

  @override
  String get clubBalanceHomeHelp =>
      'После сканирования QR покажем, на сколько времени хватит баланса на этом ПК.';

  @override
  String get zoneConversionHelp =>
      'Это время уже оплачено. Нажмите «Использовать время», чтобы запустить игру на этом ПК.';

  @override
  String get clubOffline => 'Клуб сейчас не на связи';

  @override
  String get balanceStale => 'Нет связи. Показан последний полученный остаток.';

  @override
  String balanceUpdated(String time) {
    return 'Обновлено: $time';
  }

  @override
  String get zeroMinutes => '0 мин';

  @override
  String get balancePending => 'Обновляем остаток…';

  @override
  String get useBalance => 'Использовать время';

  @override
  String balanceInZone(String zone) {
    return 'Ваше оплаченное время в зоне $zone';
  }

  @override
  String get balanceUnknown =>
      'Не удалось получить остаток. Попробуйте обновить.';

  @override
  String get clubSearchTitle => 'Выберите компьютерный клуб';

  @override
  String get clubSearchHint => 'Название клуба или адрес';

  @override
  String get clubSearchEmpty => 'Клубы не найдены. Попробуйте изменить запрос.';

  @override
  String get clubSelectZonePc => 'Выберите зону и свободный ПК';

  @override
  String get clubOfflineDetail =>
      'Клуб сейчас не на связи. Список ПК может быть неактуальным.';

  @override
  String freePcs(int count) {
    return '$count свободных';
  }

  @override
  String availablePcsCount(int available, int total) {
    return '$available из $total свободных ПК';
  }

  @override
  String get noConnection => 'Нет связи';

  @override
  String get selectThisPc => 'Свободен — выбрать этот ПК';

  @override
  String get clubNoZones => 'В этом клубе пока нет доступных зон.';

  @override
  String get supportUnavailable =>
      'Поддержка временно недоступна. Попробуйте позже.';

  @override
  String get reservationUnavailable =>
      'Бронирование сейчас временно недоступно. Попробуйте позже.';

  @override
  String get publicOffer => 'Публичная оферта';

  @override
  String get publicOfferTitle => 'Публичная оферта ClubPay';

  @override
  String get publicOfferDescription =>
      'Условия использования сервиса и игрового времени.';

  @override
  String get support => 'Поддержка';

  @override
  String get supportMessageSent => 'Сообщение отправлено в поддержку.';

  @override
  String get supportHelp =>
      'Опишите проблему: клуб, компьютер и что произошло. Сообщение сразу попадёт в поддержку. Мы ответим по указанному в обращении способу связи.';

  @override
  String get supportMessageHint => 'Напишите сообщение';

  @override
  String get send => 'Отправить';

  @override
  String get endSessionConfirmTitle => 'Завершить сеанс?';

  @override
  String get endSessionConfirmBody =>
      'Игра на этом ПК будет закрыта. Неиспользованное время сохранится в балансе клуба.';

  @override
  String get cancel => 'Отмена';

  @override
  String get end => 'Завершить';

  @override
  String get activeSession => 'Активная сессия';

  @override
  String get remaining => 'осталось';

  @override
  String get extendSession => 'Продлить сеанс';

  @override
  String get endSession => 'Завершить сеанс';

  @override
  String get ownReservation => 'Ваша бронь';

  @override
  String get ownActiveSession => 'Ваша активная сессия';

  @override
  String get reservePc => 'Забронировать ПК';

  @override
  String get extendFor => 'Продлить на';

  @override
  String get startGame => 'Начать игру';

  @override
  String get reservationStartHelp =>
      'Вы на месте? Начните игру в ClubPay — затем выберите оплату или используйте уже оплаченное время.';

  @override
  String reservationStartAvailableAt(String time) {
    return 'Кнопка «Начать игру» станет доступна в $time — за 15 минут до начала брони.';
  }

  @override
  String get reservationNoLongerAvailable =>
      'Время для начала игры закончилось. Бронь больше недоступна.';

  @override
  String get changeReservation => 'Изменить бронь';

  @override
  String get reservationCancelled => 'Бронь отменена.';

  @override
  String get cancelReservation => 'Отменить бронь';

  @override
  String get supportTooltip => 'Поддержка';

  @override
  String get searchClubsTooltip => 'Поиск клубов';

  @override
  String get openClubsMap => 'Открыть карту клубов';

  @override
  String activeSessionWithClub(String club) {
    return 'Активная сессия · $club';
  }

  @override
  String get activeSessionHelp =>
      'Нажмите, чтобы продлить или завершить сеанс.';

  @override
  String reservationWithClub(String club) {
    return 'Ваша бронь · $club';
  }

  @override
  String get reservationHeldHelp =>
      'ПК зарезервирован для вас. Откройте бронь и нажмите «Начать игру», чтобы выбрать оплату или использовать уже оплаченное время.';

  @override
  String get reservationUpcomingHelp =>
      'ПК будет отмечен как забронированный за 15 минут до начала. В это время в ClubPay станет доступна кнопка «Начать игру».';

  @override
  String get wakeSent =>
      'Команда на включение отправлена. Обновим список, когда ПК появится в сети.';

  @override
  String get removeFavorite => 'Убрать из избранного';

  @override
  String get addFavorite => 'Добавить в избранное';

  @override
  String get wake => 'Включить';

  @override
  String get favorites => 'Избранное';

  @override
  String get favoritesEmpty => 'Добавьте клуб в избранное — он появится здесь.';

  @override
  String reservedByMe(String when) {
    return 'Ваша бронь$when';
  }

  @override
  String reserved(String when) {
    return 'Забронирован$when';
  }

  @override
  String get reservationInvalidHours => 'Введите целое число часов от 1 до 24.';

  @override
  String get reservationMinimumLead =>
      'Бронь можно оформить минимум за 15 минут.';

  @override
  String get reservationRescheduled => 'Бронь перенесена';

  @override
  String get reservationCreated => 'Бронь оформлена';

  @override
  String get rescheduleReservation => 'Перенести бронь';

  @override
  String get reserve => 'Забронировать';

  @override
  String get reservationChooseStart => 'Когда хотите начать';

  @override
  String get reservationHeldIn =>
      'ПК будет отмечен как забронированный за 15 минут';

  @override
  String get reservationChooseHours => 'Сколько часов играть';

  @override
  String get hours => 'часов';

  @override
  String get reservationHoursExample => 'Например, 6';

  @override
  String get reservationPaymentInfo =>
      'Деньги сейчас не списываем. Придите к выбранному времени и начните игру на этом ПК — оплатите клубу или используйте уже оплаченное время.';

  @override
  String get reservationStartInfo =>
      'За 15 минут до начала ПК будет заблокирован для вашей брони. Откройте бронь в ClubPay и нажмите «Начать игру». Если не начать игру в течение 15 минут после начала, бронь отменится.';

  @override
  String hoursOfPlay(int hours) {
    return '$hours ч игры';
  }

  @override
  String reservationConfirmationInfo(String time) {
    return 'ПК будет отмечен как занятый с $time. В это время в ClubPay появится кнопка «Начать игру» — она откроет обычный выбор оплаты или запуск по уже оплаченному времени.';
  }

  @override
  String get toReservation => 'К брони';

  @override
  String get locationUnavailable =>
      'Не удалось определить геопозицию. Проверьте разрешение.';

  @override
  String get clubsOnMap => 'Клубы на карте';

  @override
  String get yandexMapUnavailable => 'Карта Яндекс временно не настроена.';

  @override
  String get openClub => 'Открыть клуб';

  @override
  String get legalConsent => 'Вводя номер, вы соглашаетесь с публичной офертой';

  @override
  String get otpSafety => 'Код отправлен в SMS. Не сообщайте его другим людям.';
}
