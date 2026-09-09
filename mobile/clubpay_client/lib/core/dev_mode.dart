import 'package:flutter/foundation.dart';

// Explicit debug build only. Production builds cannot display sandbox OTPs.
final localOtpTestMode =
    kDebugMode &&
    const bool.fromEnvironment('LOCAL_OTP_TEST_MODE') &&
    {'localhost', '127.0.0.1'}.contains(
      Uri.tryParse(const String.fromEnvironment('API_BASE_URL'))?.host,
    );
