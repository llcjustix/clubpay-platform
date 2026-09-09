import 'dart:typed_data';
import 'package:image/image.dart' as img;
import 'package:zxing2/qrcode.dart';

// Pure Dart decoder also supports Chrome, unlike native analyzeImage APIs.
String? decodeQrImage(Uint8List bytes) {
  if (bytes.length > 5 * 1024 * 1024) {
    throw const FormatException('image_too_large');
  }
  final decoder = img.findDecoderForData(bytes);
  final info = decoder?.startDecode(bytes);
  if (info == null || info.width * info.height > 16000000) {
    throw const FormatException('image_too_large');
  }
  final picture = img.decodeImage(bytes);
  if (picture == null) return null;
  final scaled = picture.width > 1600 || picture.height > 1600
      ? img.copyResize(
          picture,
          width: picture.width >= picture.height ? 1600 : null,
          height: picture.height > picture.width ? 1600 : null,
        )
      : picture;
  final pixels = Int32List(scaled.width * scaled.height);
  for (var y = 0; y < scaled.height; y++) {
    for (var x = 0; x < scaled.width; x++) {
      final p = scaled.getPixel(x, y);
      pixels[y * scaled.width + x] =
          (255 << 24) | (p.r.toInt() << 16) | (p.g.toInt() << 8) | p.b.toInt();
    }
  }
  try {
    return QRCodeReader()
        .decode(
          BinaryBitmap(
            HybridBinarizer(
              RGBLuminanceSource(scaled.width, scaled.height, pixels),
            ),
          ),
        )
        .text;
  } on ReaderException {
    return null;
  }
}
