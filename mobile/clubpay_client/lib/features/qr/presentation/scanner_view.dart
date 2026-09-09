import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:mobile_scanner/mobile_scanner.dart';
import '../../../core/club_theme.dart';
import '../../../core/ui.dart';

// Test seam at the platform boundary; production always creates the camera.
final scannerBuilderProvider = Provider<Widget Function(ValueChanged<String>)>(
  (ref) =>
      (onCode) => ScannerView(onCode: onCode),
);

class ScannerView extends StatefulWidget {
  const ScannerView({super.key, required this.onCode});
  final ValueChanged<String> onCode;
  @override
  State<ScannerView> createState() => _ScannerViewState();
}

class _ScannerViewState extends State<ScannerView> {
  bool _handled = false;
  int _attempt = 0;
  // Let MobileScanner own its controller: it automatically starts, observes
  // app lifecycle, and stops/disposes the camera when removed from the tree.
  @override
  Widget build(BuildContext context) => MobileScanner(
    key: ValueKey(_attempt),
    fit: BoxFit.cover,
    onDetect: (capture) {
      if (_handled) return;
      for (final code in capture.barcodes) {
        if (code.format == BarcodeFormat.qrCode && code.rawValue != null) {
          _handled = true;
          widget.onCode(code.rawValue!);
          break;
        }
      }
    },
    placeholderBuilder: (context) => Center(
      child: Padding(
        padding: const EdgeInsets.all(28),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const CupertinoActivityIndicator(color: ClubColors.text),
            const SizedBox(height: 20),
            Text(
              context.l.cameraStarting,
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 10),
            Text(
              context.l.cameraPermissionHelp,
              textAlign: TextAlign.center,
              style: const TextStyle(color: ClubColors.muted, height: 1.5),
            ),
          ],
        ),
      ),
    ),
    overlayBuilder: (context, constraints) => IgnorePointer(
      child: Center(
        child: SizedBox.square(
          dimension: constraints.maxWidth.clamp(0, 320) * .78,
          child: CustomPaint(painter: _ScanFrame()),
        ),
      ),
    ),
    errorBuilder: (context, error) => ColoredBox(
      color: ClubColors.background,
      child: Center(
        child: SingleChildScrollView(
          child: Padding(
            padding: const EdgeInsets.all(30),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(
                  CupertinoIcons.camera,
                  size: 42,
                  color: ClubColors.muted,
                ),
                const SizedBox(height: 18),
                Text(
                  context.l.cameraUnavailable,
                  textAlign: TextAlign.center,
                  style: Theme.of(context).textTheme.titleLarge,
                ),
                const SizedBox(height: 10),
                Text(
                  context.l.cameraFallbackHelp,
                  textAlign: TextAlign.center,
                  style: const TextStyle(color: ClubColors.muted, height: 1.5),
                ),
                const SizedBox(height: 16),
                TextButton(
                  onPressed: () => setState(() {
                    _handled = false;
                    _attempt++;
                  }),
                  child: Text(context.l.retryCamera),
                ),
              ],
            ),
          ),
        ),
      ),
    ),
  );
}

class _ScanFrame extends CustomPainter {
  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = ClubColors.text
      ..strokeWidth = 4
      ..style = PaintingStyle.stroke
      ..strokeCap = StrokeCap.round;
    const length = 32.0;
    const radius = 20.0;
    final p = Path()
      ..moveTo(0, length)
      ..lineTo(0, radius)
      ..quadraticBezierTo(0, 0, radius, 0)
      ..lineTo(length, 0)
      ..moveTo(size.width - length, 0)
      ..lineTo(size.width - radius, 0)
      ..quadraticBezierTo(size.width, 0, size.width, radius)
      ..lineTo(size.width, length)
      ..moveTo(size.width, size.height - length)
      ..lineTo(size.width, size.height - radius)
      ..quadraticBezierTo(
        size.width,
        size.height,
        size.width - radius,
        size.height,
      )
      ..lineTo(size.width - length, size.height)
      ..moveTo(length, size.height)
      ..lineTo(radius, size.height)
      ..quadraticBezierTo(0, size.height, 0, size.height - radius)
      ..lineTo(0, size.height - length);
    canvas.drawPath(p, paint);
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}
