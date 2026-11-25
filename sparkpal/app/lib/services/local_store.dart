import 'dart:convert';

import 'package:shared_preferences/shared_preferences.dart';

import 'spark_api.dart';

/// 本地存储：服务器列表、token、主题。
class LocalStore {
  static const _kServers = 'sparkpal.servers';
  static const _kCurrent = 'sparkpal.current';
  static const _kTokenPrefix = 'sparkpal.token.';
  static const _kTheme = 'sparkpal.theme';

  static SharedPreferences? _sp;

  static Future<SharedPreferences> _prefs() async =>
      _sp ??= await SharedPreferences.getInstance();

  /// 官方地址（没有域名，IP 直连）。
  static const String officialAddress = 'https://114.66.28.185:48285';

  static ServerEntry get officialEntry =>
      ServerEntry(address: officialAddress, label: '伙伴官方', official: true);

  static Future<List<ServerEntry>> servers() async {
    final p = await _prefs();
    final raw = p.getString(_kServers);
    final list = <ServerEntry>[];
    if (raw != null && raw.isNotEmpty) {
      try {
        for (final e in jsonDecode(raw) as List) {
          list.add(ServerEntry.fromJson((e as Map).cast<String, dynamic>()));
        }
      } catch (_) {}
    }
    // 官方永远排第一，且不允许删
    list.removeWhere((e) => e.official);
    return [officialEntry, ...list];
  }

  static Future<void> addServer(ServerEntry e) async {
    final p = await _prefs();
    final all = (await servers()).where((x) => !x.official).toList();
    all.removeWhere((x) => x.normalized == e.normalized);
    all.add(e);
    await p.setString(_kServers, jsonEncode(all.map((x) => x.toJson()).toList()));
  }

  static Future<void> removeServer(ServerEntry e) async {
    final p = await _prefs();
    final all = (await servers()).where((x) => !x.official).toList();
    all.removeWhere((x) => x.normalized == e.normalized);
    await p.setString(_kServers, jsonEncode(all.map((x) => x.toJson()).toList()));
  }

  static Future<String?> currentAddress() async {
    final p = await _prefs();
    return p.getString(_kCurrent);
  }

  static Future<void> setCurrent(String address) async {
    final p = await _prefs();
    await p.setString(_kCurrent, address);
  }

  static Future<String?> token(String address) async {
    final p = await _prefs();
    return p.getString('$_kTokenPrefix$address');
  }

  static Future<void> setToken(String address, String? token) async {
    final p = await _prefs();
    if (token == null || token.isEmpty) {
      await p.remove('$_kTokenPrefix$address');
    } else {
      await p.setString('$_kTokenPrefix$address', token);
    }
  }

  static Future<void> clearToken(String address) => setToken(address, null);

  /// 主题：system / light / dark
  static Future<String> themeMode() async {
    final p = await _prefs();
    return p.getString(_kTheme) ?? 'system';
  }

  static Future<void> setThemeMode(String mode) async {
    final p = await _prefs();
    await p.setString(_kTheme, mode);
  }
}
