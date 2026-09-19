// ignore: unused_import
import 'package:intl/intl.dart' as intl;
import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for Uzbek (`uz`).
class AppLocalizationsUz extends AppLocalizations {
  AppLocalizationsUz([String locale = 'uz']) : super(locale);

  @override
  String get appName => 'ClubPay';

  @override
  String get welcome => 'Vaqtingiz.\nO‘yiningiz.';

  @override
  String get intro =>
      'O‘yin vaqtini saqlash va sevimli klublaringizda o‘yinga qaytish uchun kiring.';

  @override
  String get phone => 'Telefon raqami';

  @override
  String get phoneHint => '+998 90 123 45 67';

  @override
  String get phoneError => 'O‘zbekiston raqamini kiriting: +998 va 9 ta raqam';

  @override
  String get continueLabel => 'Davom etish';

  @override
  String get telegramTitle => 'SMS kodni kiriting';

  @override
  String get telegramHelp =>
      'Olti xonali kod telefon raqamingizga yuborildi. Uni 3 daqiqa ichida kiriting.';

  @override
  String get openTelegram => 'SMSni qayta yuborish';

  @override
  String get otp => 'SMS kodi';

  @override
  String get verify => 'Kirish';

  @override
  String get newChallenge => 'Yangi kod olish';

  @override
  String get challengeExpired => 'So‘rov eskirgan. Yangi kod oling.';

  @override
  String get invalidOtp =>
      'Kod noto‘g‘ri yoki eskirgan. Tekshiring yoki yangi kod oling.';

  @override
  String get rateLimited =>
      'Urinishlar juda ko‘p. 15 daqiqadan keyin qayta urinib ko‘ring.';

  @override
  String get serviceUnavailable =>
      'SMS orqali kirish hozir ishlamayapti. Keyinroq urinib ko‘ring.';

  @override
  String get networkError =>
      'ClubPay bilan bog‘lanib bo‘lmadi. Internetni tekshiring va qayta urinib ko‘ring.';

  @override
  String get storageError =>
      'Sessiya saqlanmadi. Ma’lumotlarni saqlashga ruxsat bering va qayta kiring.';

  @override
  String get home => 'Asosiy';

  @override
  String get profile => 'Profil';

  @override
  String get scan => 'QR skanerlash';

  @override
  String get gameTime => 'O‘yin vaqti';

  @override
  String get perClub => 'Vaqt har bir klubda alohida saqlanadi.';

  @override
  String get emptyBalance => 'Hozircha o‘yin balansi yo‘q';

  @override
  String get emptyBalanceHelp =>
      'Klub kompyuteridagi QR kodni skanerlang. Profil sessiyasining ishlatilmagan vaqti shu yerda saqlanadi.';

  @override
  String get chooseClub => 'Klubni tanlang';

  @override
  String get unknownClub => 'Klub hali tanlanmagan';

  @override
  String get startBalance => 'Saqlangan vaqtdan boshlash';

  @override
  String get buyTime => 'O‘yin vaqti sotib olish';

  @override
  String get balanceHelp =>
      'Bu klubdagi vaqtdan foydalanish uchun kompyuterni QR orqali tanlang.';

  @override
  String get scanHelp => 'Kamerani kompyuterdagi ClubPay QR kodiga qarating.';

  @override
  String get qrInput => 'QR havolasi yoki kodi';

  @override
  String get checkQr => 'QR tekshirish';

  @override
  String get uploadQr => 'QR rasmini yuklash';

  @override
  String get camera => 'Kamerani ochish';

  @override
  String get cameraHelp =>
      'Kameraga ruxsat bering. Kamera ishlamasa, havolani kiriting yoki QR rasmini yuklang.';

  @override
  String get invalidQr =>
      'QR tanilmadi. Kompyuter ekranidagi ClubPay QR kodidan foydalaning.';

  @override
  String get qrNotFound =>
      'Bu QR topilmadi yoki endi mavjud emas. Havolani tekshiring yoki klub administratoridan kodni aniqlashtiring.';

  @override
  String get close => 'Yopish';

  @override
  String get pc => 'Kompyuter';

  @override
  String get zone => 'Zona';

  @override
  String get available => 'Bo‘sh';

  @override
  String get sleeping => 'Uyqu rejimida';

  @override
  String get occupied => 'Band';

  @override
  String get maintenance => 'Mavjud emas';

  @override
  String get packages => 'O‘yin vaqtini tanlang';

  @override
  String get customAmount => 'O‘z summangiz, so‘m';

  @override
  String get customHelp => 'Ushbu kompyuterdagi o‘yin vaqti uchun to‘lov.';

  @override
  String get providers => 'To‘lov usuli';

  @override
  String get payme => 'Payme';

  @override
  String get click => 'Click';

  @override
  String get mock => 'Sinov to‘lovi';

  @override
  String get testPayAndStart => 'Sinov to‘lovi va ishga tushirish';

  @override
  String get noProviders => 'To‘lov hozir mavjud emas';

  @override
  String get pay => 'To‘lash';

  @override
  String get clubBalance => 'Bu klubdagi vaqtingiz';

  @override
  String startTime(String time) {
    return '$time vaqtga boshlash';
  }

  @override
  String duration(int hours, int minutes, int seconds) {
    return '$hours soat $minutes daq $seconds son';
  }

  @override
  String tariffMinutes(int minutes) {
    return '$minutes daqiqa';
  }

  @override
  String tariffHours(int hours) {
    return '$hours soat';
  }

  @override
  String price(int amount) {
    return '$amount so‘m';
  }

  @override
  String zoneHourlyPrice(int amount) {
    return '$amount so‘m/soat';
  }

  @override
  String get payment => 'To‘lov va sessiya';

  @override
  String get waitingPayment => 'To‘lov kutilmoqda';

  @override
  String get startingSession => 'Sessiya boshlanmoqda';

  @override
  String get startingHelp =>
      'To‘lov tasdiqlandi. Klub kompyuteridan sessiya boshlanganligi tasdig‘ini kutyapmiz. Bu biroz vaqt olishi mumkin.';

  @override
  String get sessionReady => 'Sessiya boshlandi';

  @override
  String get sessionEnded => 'Sessiya tugadi';

  @override
  String get paymentFailed => 'To‘lov yakunlanmadi';

  @override
  String get startFailed => 'Boshlanishni tasdiqlab bo‘lmadi';

  @override
  String get checkStatus => 'Holatni qayta tekshirish';

  @override
  String get openPayment => 'To‘lov sahifasini ochish';

  @override
  String get returnHelp =>
      'To‘lovdan keyin ClubPay’ga qayting. Holat avtomatik yangilanadi.';

  @override
  String get operationPending => 'Operatsiya natijasi tekshirilmoqda';

  @override
  String get operationHelp =>
      'Takroriy to‘lov yaratmang. Holatni yana tekshiring.';

  @override
  String get language => 'Til';

  @override
  String get russian => 'Русский';

  @override
  String get uzbek => 'O‘zbekcha';

  @override
  String get logout => 'Hisobdan chiqish';

  @override
  String get name => 'Ism';

  @override
  String get refresh => 'Yangilash';

  @override
  String get back => 'Orqaga';

  @override
  String get loading => 'ClubPay yuklanmoqda';

  @override
  String get resumePayment => 'Sessiyani tekshirishni davom ettirish';

  @override
  String get noName => 'Ism kiritilmagan';

  @override
  String get scanToBuy =>
      'Vaqtni tanlash va to‘lash uchun kompyuter QR kodini skanerlang.';

  @override
  String get noTime => 'Saqlangan vaqt yo‘q';

  @override
  String get imageTooLarge => '5 MB gacha PNG yoki JPEG tanlang.';

  @override
  String get amountError => 'Noldan katta butun summa kiriting.';

  @override
  String get checkSlow =>
      'Tasdiqlash ko‘proq vaqt olyapti. Holatni qayta tekshiring.';

  @override
  String get sessionExpired => 'Kirish sessiyasi tugadi. Qayta kiring.';

  @override
  String get paymentUnavailable =>
      'Operatsiya bajarilmadi. Kompyuter ma’lumotlarini yangilang.';

  @override
  String get authRetry => 'Qayta kirish';

  @override
  String get signInHelp =>
      'Tasdiqlash kodi SMS orqali yuboriladi. Saqlangan vaqtingiz kirgandan keyin ko‘rinadi.';

  @override
  String get readyToPlay => 'O‘ynashga tayyormisiz?';

  @override
  String get welcomeBack => 'Yana xush kelibsiz';

  @override
  String get homeSubtitle => 'Kompyuteringiz. Vaqtingiz. Bir skan bilan.';

  @override
  String get accountSubtitle => 'Hisobingiz va ilova sozlamalari.';

  @override
  String get yourAccount => 'Hisobingiz';

  @override
  String get aboutApp => 'Ilova haqida';

  @override
  String get startWithQr => 'Kompyuterdagi QR dan boshlang';

  @override
  String get tapQrBelow =>
      'Pastdagi QR tugmasini bosing va kamerani bo‘sh kompyuterning QR kodiga qarating.';

  @override
  String get timeReturnsHere =>
      'Klubdagi kompyuter QR kodini skanerlang. O‘yindan keyin saqlangan balans shu yerda ko‘rinadi.';

  @override
  String get howItWorks => 'Bu qanday ishlaydi';

  @override
  String get guideScan => 'Klub va kompyuterni tanlang';

  @override
  String get guideScanHelp =>
      'Klubni toping, zonani va bo‘sh kompyuterni tanlang.';

  @override
  String get guideChoose => 'O‘yin vaqtini tanlang';

  @override
  String get guideChooseHelp =>
      'Vaqt sotib oling yoki shu klubdagi qoldiqdan foydalaning. Boshqa zonada vaqt uning narxiga qarab qayta hisoblanadi.';

  @override
  String get guidePlay => 'O‘ynang. Qolgan vaqt saqlanadi';

  @override
  String get guidePlayHelp =>
      'Seans boshlanganini kuting. Erta tugatsangiz, qolgan vaqt profilingizga qaytadi.';

  @override
  String get gotIt => 'Tushunarli';

  @override
  String get cameraUnavailable => 'Kamera mavjud emas';

  @override
  String get cameraFallbackHelp =>
      'Ilova yoki brauzer sozlamalarida kameraga kirishga ruxsat bering, so‘ng qayta urinib ko‘ring.';

  @override
  String get retryCamera => 'Qayta urinish';

  @override
  String get checkingQr => 'Kompyuter tekshirilmoqda…';

  @override
  String get scanAgain => 'Qayta skanerlash';

  @override
  String get pointCamera => 'Kamerani kompyuter QR kodiga qarating';

  @override
  String get scanAutomatic => 'U avtomatik aniqlanadi.';

  @override
  String get qrPhoto => 'QR rasmi';

  @override
  String get manualQr => 'Kod kiritish';

  @override
  String get manualQrHelp =>
      'ClubPay kompyuterining QR kodidagi havola yoki kodni kiriting.';

  @override
  String get cameraStarting => 'Kamera ulanmoqda';

  @override
  String get cameraPermissionHelp =>
      'So‘rov paydo bo‘lganda kameraga kirishga ruxsat bering.';

  @override
  String get qrCatalogUnavailable =>
      'QR tekshirilmadi: klub serveri hozir mavjud emas. Kod eskirgan bo‘lishi shart emas. Keyinroq qayta tekshiring.';

  @override
  String get retryQr => 'Qayta tekshirish';

  @override
  String get liveCatalogPreview => 'Haqiqiy kompyuter · ko‘rish';

  @override
  String get liveCatalogPreviewHelp =>
      'Kompyuter ma’lumotlari haqiqiy klubdan olindi. Siz sinov profilidasiz: kompyuter va tariflarni ko‘rish mumkin, haqiqiy to‘lov va qolgan vaqt esa mavjud emas.';

  @override
  String get myClubs => 'Mening klublarim';

  @override
  String get clubGameBalance => 'O‘yin balansi';

  @override
  String clubGameBalanceWithAmount(String amount) {
    return 'O‘yin balansi: $amount';
  }

  @override
  String get currencySuffix => 'so‘m';

  @override
  String get clubBalanceHelp => 'Uni shu klubdagi o‘yin vaqti uchun ishlating';

  @override
  String get clubBalanceHomeHelp =>
      'QR skanerlangandan keyin bu kompyuterda balans qancha vaqtga yetishini ko‘rsatamiz.';

  @override
  String get zoneConversionHelp =>
      'Bu vaqt allaqachon to‘langan. Shu kompyuterda o‘yinni boshlash uchun «Vaqtdan foydalanish» tugmasini bosing.';

  @override
  String get clubOffline => 'Klub hozir tarmoqqa ulanmagan';

  @override
  String get balanceStale => 'Aloqa yo‘q. Oxirgi olingan qoldiq ko‘rsatilgan.';

  @override
  String balanceUpdated(String time) {
    return 'Yangilangan: $time';
  }

  @override
  String get zeroMinutes => '0 daq';

  @override
  String get balancePending => 'Qoldiq yangilanmoqda…';

  @override
  String get useBalance => 'Vaqtdan foydalanish';

  @override
  String balanceInZone(String zone) {
    return '$zone zonasidagi to‘langan vaqtingiz';
  }

  @override
  String get balanceUnknown => 'Qoldiqni olish imkonsiz. Yangilab ko‘ring.';

  @override
  String get clubSearchTitle => 'Kompyuter klubini tanlang';

  @override
  String get clubSearchHint => 'Klub nomi yoki manzili';

  @override
  String get clubSearchEmpty =>
      'Klublar topilmadi. So‘rovni o‘zgartirib ko‘ring.';

  @override
  String get clubSelectZonePc => 'Zonani va bo‘sh kompyuterni tanlang';

  @override
  String get clubOfflineDetail =>
      'Klub hozir aloqada emas. Kompyuterlar ro‘yxati eskirgan bo‘lishi mumkin.';

  @override
  String freePcs(int count) {
    return '$count ta bo‘sh';
  }

  @override
  String availablePcsCount(int available, int total) {
    return '$total tadan $available ta bo‘sh kompyuter';
  }

  @override
  String get noConnection => 'Aloqa yo‘q';

  @override
  String get selectThisPc => 'Bo‘sh — shu kompyuterni tanlash';

  @override
  String get clubNoZones => 'Bu klubda hozircha mavjud zonalar yo‘q.';

  @override
  String get supportUnavailable =>
      'Qo‘llab-quvvatlash vaqtincha ishlamayapti. Keyinroq urinib ko‘ring.';

  @override
  String get reservationUnavailable =>
      'Bron qilish hozir vaqtincha ishlamayapti. Keyinroq urinib ko‘ring.';

  @override
  String get publicOffer => 'Ommaviy oferta';

  @override
  String get publicOfferTitle => 'ClubPay ommaviy ofertasi';

  @override
  String get publicOfferDescription =>
      'Xizmat va o‘yin vaqtidan foydalanish shartlari.';

  @override
  String get support => 'Yordam';

  @override
  String get supportMessageSent => 'Xabaringiz yordam xizmatiga yuborildi.';

  @override
  String get supportHelp =>
      'Muammoni yozing: klub, kompyuter va nima bo‘lganini ayting. Xabar darhol yordam xizmatiga yuboriladi. Murojaatda ko‘rsatilgan aloqa usuli orqali javob beramiz.';

  @override
  String get supportMessageHint => 'Xabar yozing';

  @override
  String get send => 'Yuborish';

  @override
  String get endSessionConfirmTitle => 'Seansni yakunlaysizmi?';

  @override
  String get endSessionConfirmBody =>
      'Ushbu kompyuterdagi o‘yin yopiladi. Ishlatilmagan vaqt klub balansida saqlanadi.';

  @override
  String get cancel => 'Bekor qilish';

  @override
  String get end => 'Yakunlash';

  @override
  String get activeSession => 'Faol seans';

  @override
  String get remaining => 'qoldi';

  @override
  String get extendSession => 'Seansni uzaytirish';

  @override
  String get endSession => 'Seansni yakunlash';

  @override
  String get ownReservation => 'Sizning broningiz';

  @override
  String get ownActiveSession => 'Sizning faol seansingiz';

  @override
  String get reservePc => 'Kompyuterni bron qilish';

  @override
  String get extendFor => 'Uzaytirish';

  @override
  String get startGame => 'O‘yinni boshlash';

  @override
  String get reservationStartHelp =>
      'Joyingizdami? ClubPay’da o‘yinni boshlang, so‘ng to‘lovni tanlang yoki avval to‘langan vaqtdan foydalaning.';

  @override
  String reservationStartAvailableAt(String time) {
    return '«O‘yinni boshlash» tugmasi bron boshlanishidan 15 daqiqa oldin, $time da faol bo‘ladi.';
  }

  @override
  String get reservationNoLongerAvailable =>
      'O‘yinni boshlash vaqti tugadi. Bron endi mavjud emas.';

  @override
  String get changeReservation => 'Bronni o‘zgartirish';

  @override
  String get reservationCancelled => 'Bron bekor qilindi.';

  @override
  String get cancelReservation => 'Bronni bekor qilish';

  @override
  String get supportTooltip => 'Yordam';

  @override
  String get searchClubsTooltip => 'Klublarni qidirish';

  @override
  String get openClubsMap => 'Klublar xaritasini ochish';

  @override
  String activeSessionWithClub(String club) {
    return 'Faol seans · $club';
  }

  @override
  String get activeSessionHelp =>
      'Seansni uzaytirish yoki yakunlash uchun bosing.';

  @override
  String reservationWithClub(String club) {
    return 'Sizning broningiz · $club';
  }

  @override
  String get reservationHeldHelp =>
      'Kompyuter siz uchun band qilingan. Bronni ochib «O‘yinni boshlash» tugmasini bosing, so‘ng to‘lovni tanlang yoki avval to‘langan vaqtdan foydalaning.';

  @override
  String get reservationUpcomingHelp =>
      'Kompyuter bron boshlanishidan 15 daqiqa oldin band qilinadi. Shu vaqtda ClubPay’da «O‘yinni boshlash» tugmasi paydo bo‘ladi.';

  @override
  String get wakeSent =>
      'Yoqish buyrug‘i yuborildi. Kompyuter tarmoqqa chiqqanda ro‘yxat yangilanadi.';

  @override
  String get removeFavorite => 'Sevimlilardan olib tashlash';

  @override
  String get addFavorite => 'Sevimlilarga qo‘shish';

  @override
  String get wake => 'Yoqish';

  @override
  String get favorites => 'Sevimlilar';

  @override
  String get favoritesEmpty =>
      'Klubni sevimlilarga qo‘shing — u shu yerda ko‘rinadi.';

  @override
  String reservedByMe(String when) {
    return 'Sizning broningiz$when';
  }

  @override
  String reserved(String when) {
    return 'Band qilingan$when';
  }

  @override
  String get reservationInvalidHours =>
      '1 dan 24 gacha bo‘lgan butun soat sonini kiriting.';

  @override
  String get reservationMinimumLead =>
      'Bronni kamida 15 daqiqa oldin rasmiylashtirish mumkin.';

  @override
  String get reservationRescheduled => 'Bron ko‘chirildi';

  @override
  String get reservationCreated => 'Bron rasmiylashtirildi';

  @override
  String get rescheduleReservation => 'Bronni ko‘chirish';

  @override
  String get reserve => 'Bron qilish';

  @override
  String get reservationChooseStart => 'Qachon boshlamoqchisiz';

  @override
  String get reservationHeldIn =>
      'Kompyuter 15 daqiqa oldin bron qilingan deb belgilanadi';

  @override
  String get reservationChooseHours => 'Necha soat o‘ynaysiz';

  @override
  String get hours => 'soat';

  @override
  String get reservationHoursExample => 'Masalan, 6';

  @override
  String get reservationPaymentInfo =>
      'Hozir pul yechilmaydi. Tanlangan vaqtda keling va shu kompyuterda o‘yinni boshlang — klubga to‘lang yoki avval to‘langan vaqtdan foydalaning.';

  @override
  String get reservationStartInfo =>
      'Boshlanishdan 15 daqiqa oldin kompyuter sizning broningiz uchun band qilinadi. ClubPay’da bronni ochib «O‘yinni boshlash» tugmasini bosing. Boshlangandan keyin 15 daqiqa ichida o‘yin boshlanmasa, bron bekor qilinadi.';

  @override
  String hoursOfPlay(int hours) {
    return '$hours soat o‘yin';
  }

  @override
  String reservationConfirmationInfo(String time) {
    return 'Kompyuter $time dan band deb belgilanadi. Shu vaqtda ClubPay’da «O‘yinni boshlash» tugmasi paydo bo‘ladi — u odatiy to‘lov tanlovi yoki avval to‘langan vaqtdan foydalanishni ochadi.';
  }

  @override
  String get toReservation => 'Bronga o‘tish';

  @override
  String get locationUnavailable =>
      'Joylashuvni aniqlab bo‘lmadi. Ruxsatni tekshiring.';

  @override
  String get clubsOnMap => 'Xaritadagi klublar';

  @override
  String get yandexMapUnavailable => 'Yandex xaritasi vaqtincha sozlanmagan.';

  @override
  String get openClub => 'Klubni ochish';

  @override
  String get legalConsent =>
      'Raqamni kiritib, ommaviy ofertaga rozilik bildirasiz';

  @override
  String get otpSafety =>
      'Kod SMS orqali yuborildi. Uni boshqa odamlarga aytmang.';
}
