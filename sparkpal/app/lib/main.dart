import 'package:flutter/material.dart';

import 'pages/gate_page.dart';
import 'pages/server_pick_page.dart';
import 'services/local_store.dart';
import 'services/spark_api.dart';
import 'theme/app_theme.dart';

void main() {
  runApp(const SparkPalApp());
}

class SparkPalApp extends StatefulWidget {
  const SparkPalApp({super.key});

  @override
  State<SparkPalApp> createState() => _SparkPalAppState();
}

class _SparkPalAppState extends State<SparkPalApp> with WidgetsBindingObserver {
  ThemeMode _mode = ThemeMode.system;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _boot();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  void didChangePlatformBrightness() => setState(() {});

  Future<void> _boot() async {
    final t = await LocalStore.themeMode();
    final addr = await LocalStore.currentAddress();
    if (!mounted) return;
    setState(() => _mode = _parseMode(t));

    // 已选过服务器：直接探测并进流程
    if (addr != null && addr.isNotEmpty) {
      try {
        final meta = await SparkApi.probe(addr);
        if (!mounted) return;
        _initial = GatePage(address: addr, meta: meta);
        setState(() {});
        return;
      } catch (_) {
        // 探测失败就回到选择页
      }
    }
    if (!mounted) return;
    setState(() => _initial = null);
  }

  ThemeMode _parseMode(String s) => switch (s) {
        'light' => ThemeMode.light,
        'dark' => ThemeMode.dark,
        _ => ThemeMode.system,
      };

  Widget? _initial;

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: '伙伴',
      debugShowCheckedModeBanner: false,
      theme: Sp.theme(false),
      darkTheme: Sp.theme(true),
      themeMode: _mode,
      themeAnimationDuration: const Duration(milliseconds: 200),
      home: _initial ?? const ServerPickPage(),
    );
  }
}
