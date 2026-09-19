import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:intl/intl.dart' as intl;

import 'app_localizations_ru.dart';
import 'app_localizations_uz.dart';

// ignore_for_file: type=lint

/// Callers can lookup localized strings with an instance of AppLocalizations
/// returned by `AppLocalizations.of(context)`.
///
/// Applications need to include `AppLocalizations.delegate()` in their app's
/// `localizationDelegates` list, and the locales they support in the app's
/// `supportedLocales` list. For example:
///
/// ```dart
/// import 'generated/app_localizations.dart';
///
/// return MaterialApp(
///   localizationsDelegates: AppLocalizations.localizationsDelegates,
///   supportedLocales: AppLocalizations.supportedLocales,
///   home: MyApplicationHome(),
/// );
/// ```
///
/// ## Update pubspec.yaml
///
/// Please make sure to update your pubspec.yaml to include the following
/// packages:
///
/// ```yaml
/// dependencies:
///   # Internationalization support.
///   flutter_localizations:
///     sdk: flutter
///   intl: any # Use the pinned version from flutter_localizations
///
///   # Rest of dependencies
/// ```
///
/// ## iOS Applications
///
/// iOS applications define key application metadata, including supported
/// locales, in an Info.plist file that is built into the application bundle.
/// To configure the locales supported by your app, you’ll need to edit this
/// file.
///
/// First, open your project’s ios/Runner.xcworkspace Xcode workspace file.
/// Then, in the Project Navigator, open the Info.plist file under the Runner
/// project’s Runner folder.
///
/// Next, select the Information Property List item, select Add Item from the
/// Editor menu, then select Localizations from the pop-up menu.
///
/// Select and expand the newly-created Localizations item then, for each
/// locale your application supports, add a new item and select the locale
/// you wish to add from the pop-up menu in the Value field. This list should
/// be consistent with the languages listed in the AppLocalizations.supportedLocales
/// property.
abstract class AppLocalizations {
  AppLocalizations(String locale)
    : localeName = intl.Intl.canonicalizedLocale(locale.toString());

  final String localeName;

  static AppLocalizations of(BuildContext context) {
    return Localizations.of<AppLocalizations>(context, AppLocalizations)!;
  }

  static const LocalizationsDelegate<AppLocalizations> delegate =
      _AppLocalizationsDelegate();

  /// A list of this localizations delegate along with the default localizations
  /// delegates.
  ///
  /// Returns a list of localizations delegates containing this delegate along with
  /// GlobalMaterialLocalizations.delegate, GlobalCupertinoLocalizations.delegate,
  /// and GlobalWidgetsLocalizations.delegate.
  ///
  /// Additional delegates can be added by appending to this list in
  /// MaterialApp. This list does not have to be used at all if a custom list
  /// of delegates is preferred or required.
  static const List<LocalizationsDelegate<dynamic>> localizationsDelegates =
      <LocalizationsDelegate<dynamic>>[
        delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
      ];

  /// A list of this localizations delegate's supported locales.
  static const List<Locale> supportedLocales = <Locale>[
    Locale('ru'),
    Locale('uz'),
  ];

  /// No description provided for @appName.
  ///
  /// In ru, this message translates to:
  /// **'ClubPay'**
  String get appName;

  /// No description provided for @welcome.
  ///
  /// In ru, this message translates to:
  /// **'Твоё время.\nТвоя игра.'**
  String get welcome;

  /// No description provided for @intro.
  ///
  /// In ru, this message translates to:
  /// **'Войдите, чтобы сохранять игровое время и возвращаться к игре в любимых клубах.'**
  String get intro;

  /// No description provided for @phone.
  ///
  /// In ru, this message translates to:
  /// **'Номер телефона'**
  String get phone;

  /// No description provided for @phoneHint.
  ///
  /// In ru, this message translates to:
  /// **'+998 90 123 45 67'**
  String get phoneHint;

  /// No description provided for @phoneError.
  ///
  /// In ru, this message translates to:
  /// **'Введите номер Узбекистана: +998 и 9 цифр'**
  String get phoneError;

  /// No description provided for @continueLabel.
  ///
  /// In ru, this message translates to:
  /// **'Продолжить'**
  String get continueLabel;

  /// No description provided for @telegramTitle.
  ///
  /// In ru, this message translates to:
  /// **'Введите код из SMS'**
  String get telegramTitle;

  /// No description provided for @telegramHelp.
  ///
  /// In ru, this message translates to:
  /// **'Мы отправили шестизначный код на ваш номер. Введите его в течение 3 минут.'**
  String get telegramHelp;

  /// No description provided for @openTelegram.
  ///
  /// In ru, this message translates to:
  /// **'Отправить SMS повторно'**
  String get openTelegram;

  /// No description provided for @otp.
  ///
  /// In ru, this message translates to:
  /// **'Код из SMS'**
  String get otp;

  /// No description provided for @verify.
  ///
  /// In ru, this message translates to:
  /// **'Войти'**
  String get verify;

  /// No description provided for @newChallenge.
  ///
  /// In ru, this message translates to:
  /// **'Получить новый код'**
  String get newChallenge;

  /// No description provided for @challengeExpired.
  ///
  /// In ru, this message translates to:
  /// **'Запрос устарел. Получите новый код.'**
  String get challengeExpired;

  /// No description provided for @invalidOtp.
  ///
  /// In ru, this message translates to:
  /// **'Код неверный или устарел. Проверьте его или получите новый.'**
  String get invalidOtp;

  /// No description provided for @rateLimited.
  ///
  /// In ru, this message translates to:
  /// **'Слишком много попыток. Попробуйте через 15 минут.'**
  String get rateLimited;

  /// No description provided for @serviceUnavailable.
  ///
  /// In ru, this message translates to:
  /// **'Вход по SMS сейчас недоступен. Попробуйте позже.'**
  String get serviceUnavailable;

  /// No description provided for @networkError.
  ///
  /// In ru, this message translates to:
  /// **'Не удалось связаться с ClubPay. Проверьте подключение и повторите.'**
  String get networkError;

  /// No description provided for @storageError.
  ///
  /// In ru, this message translates to:
  /// **'Не удалось сохранить сессию. Разрешите хранение данных и повторите вход.'**
  String get storageError;

  /// No description provided for @home.
  ///
  /// In ru, this message translates to:
  /// **'Главная'**
  String get home;

  /// No description provided for @profile.
  ///
  /// In ru, this message translates to:
  /// **'Профиль'**
  String get profile;

  /// No description provided for @scan.
  ///
  /// In ru, this message translates to:
  /// **'Сканировать QR'**
  String get scan;

  /// No description provided for @gameTime.
  ///
  /// In ru, this message translates to:
  /// **'Игровое время'**
  String get gameTime;

  /// No description provided for @perClub.
  ///
  /// In ru, this message translates to:
  /// **'Время хранится отдельно в каждом клубе.'**
  String get perClub;

  /// No description provided for @emptyBalance.
  ///
  /// In ru, this message translates to:
  /// **'Игрового баланса пока нет'**
  String get emptyBalance;

  /// No description provided for @emptyBalanceHelp.
  ///
  /// In ru, this message translates to:
  /// **'Отсканируйте QR на компьютере клуба. Неиспользованное время профильной сессии сохранится здесь.'**
  String get emptyBalanceHelp;

  /// No description provided for @chooseClub.
  ///
  /// In ru, this message translates to:
  /// **'Выберите клуб'**
  String get chooseClub;

  /// No description provided for @unknownClub.
  ///
  /// In ru, this message translates to:
  /// **'Клуб ещё не выбран'**
  String get unknownClub;

  /// No description provided for @startBalance.
  ///
  /// In ru, this message translates to:
  /// **'Начать с баланса'**
  String get startBalance;

  /// No description provided for @buyTime.
  ///
  /// In ru, this message translates to:
  /// **'Купить игровое время'**
  String get buyTime;

  /// No description provided for @balanceHelp.
  ///
  /// In ru, this message translates to:
  /// **'Выберите компьютер по QR, чтобы использовать время этого клуба.'**
  String get balanceHelp;

  /// No description provided for @scanHelp.
  ///
  /// In ru, this message translates to:
  /// **'Наведите камеру на QR ClubPay на компьютере.'**
  String get scanHelp;

  /// No description provided for @qrInput.
  ///
  /// In ru, this message translates to:
  /// **'Ссылка или код QR'**
  String get qrInput;

  /// No description provided for @checkQr.
  ///
  /// In ru, this message translates to:
  /// **'Проверить QR'**
  String get checkQr;

  /// No description provided for @uploadQr.
  ///
  /// In ru, this message translates to:
  /// **'Загрузить изображение QR'**
  String get uploadQr;

  /// No description provided for @camera.
  ///
  /// In ru, this message translates to:
  /// **'Открыть камеру'**
  String get camera;

  /// No description provided for @cameraHelp.
  ///
  /// In ru, this message translates to:
  /// **'Разрешите доступ к камере. Если камера недоступна, вставьте ссылку или загрузите QR.'**
  String get cameraHelp;

  /// No description provided for @invalidQr.
  ///
  /// In ru, this message translates to:
  /// **'QR не распознан. Используйте QR ClubPay с экрана компьютера.'**
  String get invalidQr;

  /// No description provided for @qrNotFound.
  ///
  /// In ru, this message translates to:
  /// **'Этот QR не найден или больше недоступен. Проверьте ссылку либо уточните код у администратора клуба.'**
  String get qrNotFound;

  /// No description provided for @close.
  ///
  /// In ru, this message translates to:
  /// **'Закрыть'**
  String get close;

  /// No description provided for @pc.
  ///
  /// In ru, this message translates to:
  /// **'Компьютер'**
  String get pc;

  /// No description provided for @zone.
  ///
  /// In ru, this message translates to:
  /// **'Зона'**
  String get zone;

  /// No description provided for @available.
  ///
  /// In ru, this message translates to:
  /// **'Доступен'**
  String get available;

  /// No description provided for @sleeping.
  ///
  /// In ru, this message translates to:
  /// **'В режиме сна'**
  String get sleeping;

  /// No description provided for @occupied.
  ///
  /// In ru, this message translates to:
  /// **'Занят'**
  String get occupied;

  /// No description provided for @maintenance.
  ///
  /// In ru, this message translates to:
  /// **'Недоступен'**
  String get maintenance;

  /// No description provided for @packages.
  ///
  /// In ru, this message translates to:
  /// **'Выберите игровое время'**
  String get packages;

  /// No description provided for @customAmount.
  ///
  /// In ru, this message translates to:
  /// **'Своя сумма, сум'**
  String get customAmount;

  /// No description provided for @customHelp.
  ///
  /// In ru, this message translates to:
  /// **'Сумма оплаты за игровое время на этом ПК.'**
  String get customHelp;

  /// No description provided for @providers.
  ///
  /// In ru, this message translates to:
  /// **'Способ оплаты'**
  String get providers;

  /// No description provided for @payme.
  ///
  /// In ru, this message translates to:
  /// **'Payme'**
  String get payme;

  /// No description provided for @click.
  ///
  /// In ru, this message translates to:
  /// **'Click'**
  String get click;

  /// No description provided for @mock.
  ///
  /// In ru, this message translates to:
  /// **'Тестовая оплата'**
  String get mock;

  /// No description provided for @testPayAndStart.
  ///
  /// In ru, this message translates to:
  /// **'Тестово оплатить и запустить'**
  String get testPayAndStart;

  /// No description provided for @noProviders.
  ///
  /// In ru, this message translates to:
  /// **'Оплата сейчас недоступна'**
  String get noProviders;

  /// No description provided for @pay.
  ///
  /// In ru, this message translates to:
  /// **'Оплатить'**
  String get pay;

  /// No description provided for @clubBalance.
  ///
  /// In ru, this message translates to:
  /// **'Ваше время в этом клубе'**
  String get clubBalance;

  /// No description provided for @startTime.
  ///
  /// In ru, this message translates to:
  /// **'Начать на {time}'**
  String startTime(String time);

  /// No description provided for @duration.
  ///
  /// In ru, this message translates to:
  /// **'{hours} ч {minutes} мин {seconds} с'**
  String duration(int hours, int minutes, int seconds);

  /// No description provided for @price.
  ///
  /// In ru, this message translates to:
  /// **'{amount} сум'**
  String price(int amount);

  /// No description provided for @zoneHourlyPrice.
  ///
  /// In ru, this message translates to:
  /// **'{amount} сум/ч'**
  String zoneHourlyPrice(int amount);

  /// No description provided for @payment.
  ///
  /// In ru, this message translates to:
  /// **'Оплата и сессия'**
  String get payment;

  /// No description provided for @waitingPayment.
  ///
  /// In ru, this message translates to:
  /// **'Ожидаем оплату'**
  String get waitingPayment;

  /// No description provided for @startingSession.
  ///
  /// In ru, this message translates to:
  /// **'Запускаем сессию'**
  String get startingSession;

  /// No description provided for @startingHelp.
  ///
  /// In ru, this message translates to:
  /// **'Оплата подтверждена. Ожидаем подтверждение запуска компьютером клуба. Это может занять некоторое время.'**
  String get startingHelp;

  /// No description provided for @sessionReady.
  ///
  /// In ru, this message translates to:
  /// **'Сессия запущена'**
  String get sessionReady;

  /// No description provided for @sessionEnded.
  ///
  /// In ru, this message translates to:
  /// **'Сессия завершена'**
  String get sessionEnded;

  /// No description provided for @paymentFailed.
  ///
  /// In ru, this message translates to:
  /// **'Оплата не завершена'**
  String get paymentFailed;

  /// No description provided for @startFailed.
  ///
  /// In ru, this message translates to:
  /// **'Не удалось подтвердить запуск'**
  String get startFailed;

  /// No description provided for @checkStatus.
  ///
  /// In ru, this message translates to:
  /// **'Повторить проверку статуса'**
  String get checkStatus;

  /// No description provided for @openPayment.
  ///
  /// In ru, this message translates to:
  /// **'Открыть страницу оплаты'**
  String get openPayment;

  /// No description provided for @returnHelp.
  ///
  /// In ru, this message translates to:
  /// **'После оплаты вернитесь в ClubPay. Статус обновится автоматически.'**
  String get returnHelp;

  /// No description provided for @operationPending.
  ///
  /// In ru, this message translates to:
  /// **'Проверяем результат операции'**
  String get operationPending;

  /// No description provided for @operationHelp.
  ///
  /// In ru, this message translates to:
  /// **'Не создавайте повторную оплату. Проверьте статус ещё раз.'**
  String get operationHelp;

  /// No description provided for @language.
  ///
  /// In ru, this message translates to:
  /// **'Язык'**
  String get language;

  /// No description provided for @russian.
  ///
  /// In ru, this message translates to:
  /// **'Русский'**
  String get russian;

  /// No description provided for @uzbek.
  ///
  /// In ru, this message translates to:
  /// **'O‘zbekcha'**
  String get uzbek;

  /// No description provided for @logout.
  ///
  /// In ru, this message translates to:
  /// **'Выйти из аккаунта'**
  String get logout;

  /// No description provided for @name.
  ///
  /// In ru, this message translates to:
  /// **'Имя'**
  String get name;

  /// No description provided for @refresh.
  ///
  /// In ru, this message translates to:
  /// **'Обновить'**
  String get refresh;

  /// No description provided for @back.
  ///
  /// In ru, this message translates to:
  /// **'Назад'**
  String get back;

  /// No description provided for @loading.
  ///
  /// In ru, this message translates to:
  /// **'Загружаем ClubPay'**
  String get loading;

  /// No description provided for @resumePayment.
  ///
  /// In ru, this message translates to:
  /// **'Продолжить проверку сессии'**
  String get resumePayment;

  /// No description provided for @noName.
  ///
  /// In ru, this message translates to:
  /// **'Имя не указано'**
  String get noName;

  /// No description provided for @scanToBuy.
  ///
  /// In ru, this message translates to:
  /// **'Отсканируйте QR компьютера, чтобы выбрать время и оплатить.'**
  String get scanToBuy;

  /// No description provided for @noTime.
  ///
  /// In ru, this message translates to:
  /// **'Нет сохранённого времени'**
  String get noTime;

  /// No description provided for @imageTooLarge.
  ///
  /// In ru, this message translates to:
  /// **'Выберите PNG или JPEG до 5 МБ.'**
  String get imageTooLarge;

  /// No description provided for @amountError.
  ///
  /// In ru, this message translates to:
  /// **'Введите целую сумму больше нуля.'**
  String get amountError;

  /// No description provided for @checkSlow.
  ///
  /// In ru, this message translates to:
  /// **'Подтверждение занимает больше времени. Повторите проверку статуса.'**
  String get checkSlow;

  /// No description provided for @sessionExpired.
  ///
  /// In ru, this message translates to:
  /// **'Сессия входа завершена. Войдите снова.'**
  String get sessionExpired;

  /// No description provided for @paymentUnavailable.
  ///
  /// In ru, this message translates to:
  /// **'Операция не выполнена. Обновите данные компьютера.'**
  String get paymentUnavailable;

  /// No description provided for @authRetry.
  ///
  /// In ru, this message translates to:
  /// **'Повторить вход'**
  String get authRetry;

  /// No description provided for @signInHelp.
  ///
  /// In ru, this message translates to:
  /// **'Мы отправим одно SMS с кодом. Ваши остатки времени будут доступны после входа.'**
  String get signInHelp;

  /// No description provided for @readyToPlay.
  ///
  /// In ru, this message translates to:
  /// **'Готовы играть?'**
  String get readyToPlay;

  /// No description provided for @welcomeBack.
  ///
  /// In ru, this message translates to:
  /// **'С возвращением'**
  String get welcomeBack;

  /// No description provided for @homeSubtitle.
  ///
  /// In ru, this message translates to:
  /// **'Ваш компьютер. Ваше время. В один скан.'**
  String get homeSubtitle;

  /// No description provided for @accountSubtitle.
  ///
  /// In ru, this message translates to:
  /// **'Ваш аккаунт и настройки приложения.'**
  String get accountSubtitle;

  /// No description provided for @yourAccount.
  ///
  /// In ru, this message translates to:
  /// **'Ваш аккаунт'**
  String get yourAccount;

  /// No description provided for @aboutApp.
  ///
  /// In ru, this message translates to:
  /// **'О приложении'**
  String get aboutApp;

  /// No description provided for @startWithQr.
  ///
  /// In ru, this message translates to:
  /// **'Начните с QR на компьютере'**
  String get startWithQr;

  /// No description provided for @tapQrBelow.
  ///
  /// In ru, this message translates to:
  /// **'Нажмите кнопку QR внизу и наведите камеру на QR свободного ПК.'**
  String get tapQrBelow;

  /// No description provided for @timeReturnsHere.
  ///
  /// In ru, this message translates to:
  /// **'Отсканируйте QR компьютера в клубе. После игры здесь появится сохранённый баланс.'**
  String get timeReturnsHere;

  /// No description provided for @howItWorks.
  ///
  /// In ru, this message translates to:
  /// **'Как это работает'**
  String get howItWorks;

  /// No description provided for @guideScan.
  ///
  /// In ru, this message translates to:
  /// **'Выберите клуб и компьютер'**
  String get guideScan;

  /// No description provided for @guideScanHelp.
  ///
  /// In ru, this message translates to:
  /// **'Найдите клуб, выберите зону и свободный компьютер.'**
  String get guideScanHelp;

  /// No description provided for @guideChoose.
  ///
  /// In ru, this message translates to:
  /// **'Выберите игровое время'**
  String get guideChoose;

  /// No description provided for @guideChooseHelp.
  ///
  /// In ru, this message translates to:
  /// **'Купите время или используйте остаток этого клуба. В другой зоне время пересчитается по её цене.'**
  String get guideChooseHelp;

  /// No description provided for @guidePlay.
  ///
  /// In ru, this message translates to:
  /// **'Играйте. Остаток сохранится'**
  String get guidePlay;

  /// No description provided for @guidePlayHelp.
  ///
  /// In ru, this message translates to:
  /// **'Дождитесь подтверждения запуска. Если закончите раньше, оставшееся время вернётся в профиль.'**
  String get guidePlayHelp;

  /// No description provided for @gotIt.
  ///
  /// In ru, this message translates to:
  /// **'Понятно'**
  String get gotIt;

  /// No description provided for @cameraUnavailable.
  ///
  /// In ru, this message translates to:
  /// **'Камера недоступна'**
  String get cameraUnavailable;

  /// No description provided for @cameraFallbackHelp.
  ///
  /// In ru, this message translates to:
  /// **'Разрешите доступ к камере в настройках приложения или браузера, затем попробуйте ещё раз.'**
  String get cameraFallbackHelp;

  /// No description provided for @retryCamera.
  ///
  /// In ru, this message translates to:
  /// **'Попробовать снова'**
  String get retryCamera;

  /// No description provided for @checkingQr.
  ///
  /// In ru, this message translates to:
  /// **'Проверяем компьютер…'**
  String get checkingQr;

  /// No description provided for @scanAgain.
  ///
  /// In ru, this message translates to:
  /// **'Сканировать ещё раз'**
  String get scanAgain;

  /// No description provided for @pointCamera.
  ///
  /// In ru, this message translates to:
  /// **'Наведите камеру на QR компьютера'**
  String get pointCamera;

  /// No description provided for @scanAutomatic.
  ///
  /// In ru, this message translates to:
  /// **'Он распознается автоматически.'**
  String get scanAutomatic;

  /// No description provided for @qrPhoto.
  ///
  /// In ru, this message translates to:
  /// **'Фото QR'**
  String get qrPhoto;

  /// No description provided for @manualQr.
  ///
  /// In ru, this message translates to:
  /// **'Ввести код'**
  String get manualQr;

  /// No description provided for @manualQrHelp.
  ///
  /// In ru, this message translates to:
  /// **'Вставьте ссылку или код из QR компьютера ClubPay.'**
  String get manualQrHelp;

  /// No description provided for @cameraStarting.
  ///
  /// In ru, this message translates to:
  /// **'Подключаем камеру'**
  String get cameraStarting;

  /// No description provided for @cameraPermissionHelp.
  ///
  /// In ru, this message translates to:
  /// **'Разрешите доступ к камере, когда появится запрос.'**
  String get cameraPermissionHelp;

  /// No description provided for @qrCatalogUnavailable.
  ///
  /// In ru, this message translates to:
  /// **'Не удалось проверить QR: сервер клуба сейчас недоступен. Код не обязательно устарел. Повторите проверку позже.'**
  String get qrCatalogUnavailable;

  /// No description provided for @retryQr.
  ///
  /// In ru, this message translates to:
  /// **'Повторить проверку'**
  String get retryQr;

  /// No description provided for @liveCatalogPreview.
  ///
  /// In ru, this message translates to:
  /// **'Рабочий ПК · просмотр'**
  String get liveCatalogPreview;

  /// No description provided for @liveCatalogPreviewHelp.
  ///
  /// In ru, this message translates to:
  /// **'Данные компьютера получены из рабочего клуба. Вы вошли в тестовый профиль: здесь можно посмотреть ПК и тарифы, но реальные оплаты и остатки времени недоступны.'**
  String get liveCatalogPreviewHelp;

  /// No description provided for @myClubs.
  ///
  /// In ru, this message translates to:
  /// **'Мои клубы'**
  String get myClubs;

  /// No description provided for @clubGameBalance.
  ///
  /// In ru, this message translates to:
  /// **'Игровой баланс'**
  String get clubGameBalance;

  /// No description provided for @clubGameBalanceWithAmount.
  ///
  /// In ru, this message translates to:
  /// **'Игровой баланс: {amount}'**
  String clubGameBalanceWithAmount(String amount);

  /// No description provided for @currencySuffix.
  ///
  /// In ru, this message translates to:
  /// **'сум'**
  String get currencySuffix;

  /// No description provided for @clubBalanceHelp.
  ///
  /// In ru, this message translates to:
  /// **'Используйте его для игрового времени в этом клубе'**
  String get clubBalanceHelp;

  /// No description provided for @clubBalanceHomeHelp.
  ///
  /// In ru, this message translates to:
  /// **'После сканирования QR покажем, на сколько времени хватит баланса на этом ПК.'**
  String get clubBalanceHomeHelp;

  /// No description provided for @zoneConversionHelp.
  ///
  /// In ru, this message translates to:
  /// **'Это время уже оплачено. Нажмите «Использовать время», чтобы запустить игру на этом ПК.'**
  String get zoneConversionHelp;

  /// No description provided for @clubOffline.
  ///
  /// In ru, this message translates to:
  /// **'Клуб сейчас не на связи'**
  String get clubOffline;

  /// No description provided for @balanceStale.
  ///
  /// In ru, this message translates to:
  /// **'Нет связи. Показан последний полученный остаток.'**
  String get balanceStale;

  /// No description provided for @balanceUpdated.
  ///
  /// In ru, this message translates to:
  /// **'Обновлено: {time}'**
  String balanceUpdated(String time);

  /// No description provided for @zeroMinutes.
  ///
  /// In ru, this message translates to:
  /// **'0 мин'**
  String get zeroMinutes;

  /// No description provided for @balancePending.
  ///
  /// In ru, this message translates to:
  /// **'Обновляем остаток…'**
  String get balancePending;

  /// No description provided for @useBalance.
  ///
  /// In ru, this message translates to:
  /// **'Использовать время'**
  String get useBalance;

  /// No description provided for @balanceInZone.
  ///
  /// In ru, this message translates to:
  /// **'Ваше оплаченное время в зоне {zone}'**
  String balanceInZone(String zone);

  /// No description provided for @balanceUnknown.
  ///
  /// In ru, this message translates to:
  /// **'Не удалось получить остаток. Попробуйте обновить.'**
  String get balanceUnknown;

  /// No description provided for @clubSearchTitle.
  ///
  /// In ru, this message translates to:
  /// **'Выберите компьютерный клуб'**
  String get clubSearchTitle;

  /// No description provided for @clubSearchHint.
  ///
  /// In ru, this message translates to:
  /// **'Название клуба или адрес'**
  String get clubSearchHint;

  /// No description provided for @clubSearchEmpty.
  ///
  /// In ru, this message translates to:
  /// **'Клубы не найдены. Попробуйте изменить запрос.'**
  String get clubSearchEmpty;

  /// No description provided for @clubSelectZonePc.
  ///
  /// In ru, this message translates to:
  /// **'Выберите зону и свободный ПК'**
  String get clubSelectZonePc;

  /// No description provided for @clubOfflineDetail.
  ///
  /// In ru, this message translates to:
  /// **'Клуб сейчас не на связи. Список ПК может быть неактуальным.'**
  String get clubOfflineDetail;

  /// No description provided for @freePcs.
  ///
  /// In ru, this message translates to:
  /// **'{count} свободных'**
  String freePcs(int count);

  /// No description provided for @noConnection.
  ///
  /// In ru, this message translates to:
  /// **'Нет связи'**
  String get noConnection;

  /// No description provided for @selectThisPc.
  ///
  /// In ru, this message translates to:
  /// **'Свободен — выбрать этот ПК'**
  String get selectThisPc;

  /// No description provided for @clubNoZones.
  ///
  /// In ru, this message translates to:
  /// **'В этом клубе пока нет доступных зон.'**
  String get clubNoZones;

  /// No description provided for @supportUnavailable.
  ///
  /// In ru, this message translates to:
  /// **'Поддержка временно недоступна. Попробуйте позже.'**
  String get supportUnavailable;

  /// No description provided for @reservationUnavailable.
  ///
  /// In ru, this message translates to:
  /// **'Бронирование сейчас временно недоступно. Попробуйте позже.'**
  String get reservationUnavailable;

  /// No description provided for @publicOffer.
  ///
  /// In ru, this message translates to:
  /// **'Публичная оферта'**
  String get publicOffer;

  /// No description provided for @publicOfferTitle.
  ///
  /// In ru, this message translates to:
  /// **'Публичная оферта ClubPay'**
  String get publicOfferTitle;

  /// No description provided for @publicOfferDescription.
  ///
  /// In ru, this message translates to:
  /// **'Условия использования сервиса и игрового времени.'**
  String get publicOfferDescription;

  /// No description provided for @support.
  ///
  /// In ru, this message translates to:
  /// **'Поддержка'**
  String get support;

  /// No description provided for @supportMessageSent.
  ///
  /// In ru, this message translates to:
  /// **'Сообщение отправлено в поддержку.'**
  String get supportMessageSent;

  /// No description provided for @supportHelp.
  ///
  /// In ru, this message translates to:
  /// **'Опишите проблему: клуб, компьютер и что произошло. Сообщение сразу попадёт в поддержку. Мы ответим по указанному в обращении способу связи.'**
  String get supportHelp;

  /// No description provided for @supportMessageHint.
  ///
  /// In ru, this message translates to:
  /// **'Напишите сообщение'**
  String get supportMessageHint;

  /// No description provided for @send.
  ///
  /// In ru, this message translates to:
  /// **'Отправить'**
  String get send;

  /// No description provided for @endSessionConfirmTitle.
  ///
  /// In ru, this message translates to:
  /// **'Завершить сеанс?'**
  String get endSessionConfirmTitle;

  /// No description provided for @endSessionConfirmBody.
  ///
  /// In ru, this message translates to:
  /// **'Игра на этом ПК будет закрыта. Неиспользованное время сохранится в балансе клуба.'**
  String get endSessionConfirmBody;

  /// No description provided for @cancel.
  ///
  /// In ru, this message translates to:
  /// **'Отмена'**
  String get cancel;

  /// No description provided for @end.
  ///
  /// In ru, this message translates to:
  /// **'Завершить'**
  String get end;

  /// No description provided for @activeSession.
  ///
  /// In ru, this message translates to:
  /// **'Активная сессия'**
  String get activeSession;

  /// No description provided for @remaining.
  ///
  /// In ru, this message translates to:
  /// **'осталось'**
  String get remaining;

  /// No description provided for @extendSession.
  ///
  /// In ru, this message translates to:
  /// **'Продлить сеанс'**
  String get extendSession;

  /// No description provided for @endSession.
  ///
  /// In ru, this message translates to:
  /// **'Завершить сеанс'**
  String get endSession;

  /// No description provided for @ownReservation.
  ///
  /// In ru, this message translates to:
  /// **'Ваша бронь'**
  String get ownReservation;

  /// No description provided for @ownActiveSession.
  ///
  /// In ru, this message translates to:
  /// **'Ваша активная сессия'**
  String get ownActiveSession;

  /// No description provided for @reservePc.
  ///
  /// In ru, this message translates to:
  /// **'Забронировать ПК'**
  String get reservePc;

  /// No description provided for @extendFor.
  ///
  /// In ru, this message translates to:
  /// **'Продлить на'**
  String get extendFor;

  /// No description provided for @startGame.
  ///
  /// In ru, this message translates to:
  /// **'Начать игру'**
  String get startGame;

  /// No description provided for @reservationStartHelp.
  ///
  /// In ru, this message translates to:
  /// **'Вы на месте? Начните игру в ClubPay — затем выберите оплату или используйте уже оплаченное время.'**
  String get reservationStartHelp;

  /// No description provided for @reservationStartAvailableAt.
  ///
  /// In ru, this message translates to:
  /// **'Кнопка «Начать игру» станет доступна в {time} — за 15 минут до начала брони.'**
  String reservationStartAvailableAt(String time);

  /// No description provided for @reservationNoLongerAvailable.
  ///
  /// In ru, this message translates to:
  /// **'Время для начала игры закончилось. Бронь больше недоступна.'**
  String get reservationNoLongerAvailable;

  /// No description provided for @changeReservation.
  ///
  /// In ru, this message translates to:
  /// **'Изменить бронь'**
  String get changeReservation;

  /// No description provided for @reservationCancelled.
  ///
  /// In ru, this message translates to:
  /// **'Бронь отменена.'**
  String get reservationCancelled;

  /// No description provided for @cancelReservation.
  ///
  /// In ru, this message translates to:
  /// **'Отменить бронь'**
  String get cancelReservation;

  /// No description provided for @supportTooltip.
  ///
  /// In ru, this message translates to:
  /// **'Поддержка'**
  String get supportTooltip;

  /// No description provided for @searchClubsTooltip.
  ///
  /// In ru, this message translates to:
  /// **'Поиск клубов'**
  String get searchClubsTooltip;

  /// No description provided for @openClubsMap.
  ///
  /// In ru, this message translates to:
  /// **'Открыть карту клубов'**
  String get openClubsMap;

  /// No description provided for @activeSessionWithClub.
  ///
  /// In ru, this message translates to:
  /// **'Активная сессия · {club}'**
  String activeSessionWithClub(String club);

  /// No description provided for @activeSessionHelp.
  ///
  /// In ru, this message translates to:
  /// **'Нажмите, чтобы продлить или завершить сеанс.'**
  String get activeSessionHelp;

  /// No description provided for @reservationWithClub.
  ///
  /// In ru, this message translates to:
  /// **'Ваша бронь · {club}'**
  String reservationWithClub(String club);

  /// No description provided for @reservationHeldHelp.
  ///
  /// In ru, this message translates to:
  /// **'ПК зарезервирован для вас. Откройте бронь и нажмите «Начать игру», чтобы выбрать оплату или использовать уже оплаченное время.'**
  String get reservationHeldHelp;

  /// No description provided for @reservationUpcomingHelp.
  ///
  /// In ru, this message translates to:
  /// **'ПК будет отмечен как забронированный за 15 минут до начала. В это время в ClubPay станет доступна кнопка «Начать игру».'**
  String get reservationUpcomingHelp;

  /// No description provided for @wakeSent.
  ///
  /// In ru, this message translates to:
  /// **'Команда на включение отправлена. Обновим список, когда ПК появится в сети.'**
  String get wakeSent;

  /// No description provided for @removeFavorite.
  ///
  /// In ru, this message translates to:
  /// **'Убрать из избранного'**
  String get removeFavorite;

  /// No description provided for @addFavorite.
  ///
  /// In ru, this message translates to:
  /// **'Добавить в избранное'**
  String get addFavorite;

  /// No description provided for @wake.
  ///
  /// In ru, this message translates to:
  /// **'Включить'**
  String get wake;

  /// No description provided for @favorites.
  ///
  /// In ru, this message translates to:
  /// **'Избранное'**
  String get favorites;

  /// No description provided for @favoritesEmpty.
  ///
  /// In ru, this message translates to:
  /// **'Добавьте клуб в избранное — он появится здесь.'**
  String get favoritesEmpty;

  /// No description provided for @reservedByMe.
  ///
  /// In ru, this message translates to:
  /// **'Ваша бронь{when}'**
  String reservedByMe(String when);

  /// No description provided for @reserved.
  ///
  /// In ru, this message translates to:
  /// **'Забронирован{when}'**
  String reserved(String when);

  /// No description provided for @reservationInvalidHours.
  ///
  /// In ru, this message translates to:
  /// **'Введите целое число часов от 1 до 24.'**
  String get reservationInvalidHours;

  /// No description provided for @reservationMinimumLead.
  ///
  /// In ru, this message translates to:
  /// **'Бронь можно оформить минимум за 15 минут.'**
  String get reservationMinimumLead;

  /// No description provided for @reservationRescheduled.
  ///
  /// In ru, this message translates to:
  /// **'Бронь перенесена'**
  String get reservationRescheduled;

  /// No description provided for @reservationCreated.
  ///
  /// In ru, this message translates to:
  /// **'Бронь оформлена'**
  String get reservationCreated;

  /// No description provided for @rescheduleReservation.
  ///
  /// In ru, this message translates to:
  /// **'Перенести бронь'**
  String get rescheduleReservation;

  /// No description provided for @reserve.
  ///
  /// In ru, this message translates to:
  /// **'Забронировать'**
  String get reserve;

  /// No description provided for @reservationChooseStart.
  ///
  /// In ru, this message translates to:
  /// **'Когда хотите начать'**
  String get reservationChooseStart;

  /// No description provided for @reservationHeldIn.
  ///
  /// In ru, this message translates to:
  /// **'ПК будет отмечен как забронированный за 15 минут'**
  String get reservationHeldIn;

  /// No description provided for @reservationChooseHours.
  ///
  /// In ru, this message translates to:
  /// **'Сколько часов играть'**
  String get reservationChooseHours;

  /// No description provided for @hours.
  ///
  /// In ru, this message translates to:
  /// **'часов'**
  String get hours;

  /// No description provided for @reservationHoursExample.
  ///
  /// In ru, this message translates to:
  /// **'Например, 6'**
  String get reservationHoursExample;

  /// No description provided for @reservationPaymentInfo.
  ///
  /// In ru, this message translates to:
  /// **'Деньги сейчас не списываем. Придите к выбранному времени и начните игру на этом ПК — оплатите клубу или используйте уже оплаченное время.'**
  String get reservationPaymentInfo;

  /// No description provided for @reservationStartInfo.
  ///
  /// In ru, this message translates to:
  /// **'За 15 минут до начала ПК будет заблокирован для вашей брони. Откройте бронь в ClubPay и нажмите «Начать игру». Если не начать игру в течение 15 минут после начала, бронь отменится.'**
  String get reservationStartInfo;

  /// No description provided for @hoursOfPlay.
  ///
  /// In ru, this message translates to:
  /// **'{hours} ч игры'**
  String hoursOfPlay(int hours);

  /// No description provided for @reservationConfirmationInfo.
  ///
  /// In ru, this message translates to:
  /// **'ПК будет отмечен как занятый с {time}. В это время в ClubPay появится кнопка «Начать игру» — она откроет обычный выбор оплаты или запуск по уже оплаченному времени.'**
  String reservationConfirmationInfo(String time);

  /// No description provided for @toReservation.
  ///
  /// In ru, this message translates to:
  /// **'К брони'**
  String get toReservation;

  /// No description provided for @locationUnavailable.
  ///
  /// In ru, this message translates to:
  /// **'Не удалось определить геопозицию. Проверьте разрешение.'**
  String get locationUnavailable;

  /// No description provided for @clubsOnMap.
  ///
  /// In ru, this message translates to:
  /// **'Клубы на карте'**
  String get clubsOnMap;

  /// No description provided for @yandexMapUnavailable.
  ///
  /// In ru, this message translates to:
  /// **'Карта Яндекс временно не настроена.'**
  String get yandexMapUnavailable;

  /// No description provided for @openClub.
  ///
  /// In ru, this message translates to:
  /// **'Открыть клуб'**
  String get openClub;

  /// No description provided for @legalConsent.
  ///
  /// In ru, this message translates to:
  /// **'Вводя номер, вы соглашаетесь с публичной офертой'**
  String get legalConsent;

  /// No description provided for @otpSafety.
  ///
  /// In ru, this message translates to:
  /// **'Код отправлен в SMS. Не сообщайте его другим людям.'**
  String get otpSafety;
}

class _AppLocalizationsDelegate
    extends LocalizationsDelegate<AppLocalizations> {
  const _AppLocalizationsDelegate();

  @override
  Future<AppLocalizations> load(Locale locale) {
    return SynchronousFuture<AppLocalizations>(lookupAppLocalizations(locale));
  }

  @override
  bool isSupported(Locale locale) =>
      <String>['ru', 'uz'].contains(locale.languageCode);

  @override
  bool shouldReload(_AppLocalizationsDelegate old) => false;
}

AppLocalizations lookupAppLocalizations(Locale locale) {
  // Lookup logic when only language code is specified.
  switch (locale.languageCode) {
    case 'ru':
      return AppLocalizationsRu();
    case 'uz':
      return AppLocalizationsUz();
  }

  throw FlutterError(
    'AppLocalizations.delegate failed to load unsupported locale "$locale". This is likely '
    'an issue with the localizations generation tool. Please file an issue '
    'on GitHub with a reproducible sample app and the gen-l10n configuration '
    'that was used.',
  );
}
