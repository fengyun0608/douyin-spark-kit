import 'dart:convert';

import 'package:http/http.dart' as http;

/// 服务器的能力声明。
///
/// 客户端**不会**判断「这是不是官方版」——
/// 一切以服务端 `/api/spark/meta` 返回为准。
class ServerMeta {
  final String name;
  final int apiVersion;
  final bool official;
  final bool billing;
  final bool needPairCode;
  final bool needInvite;
  final int maxAccounts;
  final int maxFriends;
  final int maxDaily;
  final String plan;

  const ServerMeta({
    required this.name,
    required this.apiVersion,
    required this.official,
    required this.billing,
    required this.needPairCode,
    required this.needInvite,
    required this.maxAccounts,
    required this.maxFriends,
    required this.maxDaily,
    required this.plan,
  });

  factory ServerMeta.fromJson(Map<String, dynamic> j) {
    final q = (j['quota'] as Map?)?.cast<String, dynamic>() ?? const {};
    int gi(String k, int def) {
      final v = q[k];
      if (v is int) return v;
      if (v is num) return v.toInt();
      return def;
    }

    return ServerMeta(
      name: (j['name'] ?? '伙伴服务器') as String,
      apiVersion: (j['apiVersion'] as num?)?.toInt() ?? 0,
      official: j['official'] == true,
      billing: j['billing'] == true,
      needPairCode: j['needPairCode'] == true,
      needInvite: j['needInvite'] == true,
      maxAccounts: gi('maxAccounts', 1),
      maxFriends: gi('maxFriends', 10),
      maxDaily: gi('maxDaily', 5),
      plan: (q['plan'] ?? 'free') as String,
    );
  }
}

/// 一个服务器条目。
class ServerEntry {
  final String address;
  final String label;
  final bool official;
  ServerEntry({required this.address, required this.label, this.official = false});

  String get normalized {
    var a = address.trim();
    if (a.isEmpty) return a;
    if (!a.startsWith('http://') && !a.startsWith('https://')) a = 'http://$a';
    while (a.endsWith('/')) {
      a = a.substring(0, a.length - 1);
    }
    return a;
  }

  Map<String, dynamic> toJson() => {'address': address, 'label': label, 'official': official};

  static ServerEntry fromJson(Map<String, dynamic> j) => ServerEntry(
        address: (j['address'] ?? '') as String,
        label: (j['label'] ?? '') as String,
        official: j['official'] == true,
      );
}

/// 接口客户端。
class SparkApi {
  final String base;
  String? token;

  SparkApi(this.base, {this.token});

  Uri _u(String path) => Uri.parse('$base/api/spark$path');

  Map<String, String> get _headers => {
        'Content-Type': 'application/json',
        if (token != null && token!.isNotEmpty) 'Authorization': 'Bearer $token',
      };

  /// 探测服务器：能连上 + 是伙伴服务 + 版本兼容。
  static Future<ServerMeta> probe(String address, {Duration? timeout}) async {
    final e = ServerEntry(address: address, label: '');
    final uri = Uri.parse('${e.normalized}/api/spark/meta');
    final r = await http
        .get(uri, headers: {'Content-Type': 'application/json'})
        .timeout(timeout ?? const Duration(seconds: 8));
    if (r.statusCode != 200) {
      throw ApiError('server_bad_status', '服务器返回 ${r.statusCode}');
    }
    final j = jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
    if (j['app'] != 'sparkpal') {
      throw ApiError('not_sparkpal', '这个地址不是伙伴服务器');
    }
    final v = (j['apiVersion'] as num?)?.toInt() ?? 0;
    if (v < 1) {
      throw ApiError('version_old', '服务器版本过旧（接口 v$v）');
    }
    return ServerMeta.fromJson(j);
  }

  Future<ServerMeta> meta() async => probe(base);

  Future<Map<String, dynamic>> _post(String path, Map<String, dynamic> body) async {
    final r = await http
        .post(_u(path), headers: _headers, body: jsonEncode(body))
        .timeout(const Duration(seconds: 20));
    final j = jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
    if (r.statusCode >= 400 || j['ok'] == false) {
      throw ApiError((j['code'] ?? 'error') as String,
          (j['message'] ?? '请求失败') as String);
    }
    return j;
  }

  /// 邀请码校验（只校验，不占用）。
  Future<Map<String, dynamic>> checkInvite(String code) =>
      _post('/auth/invite/check', {'code': code});

  Future<void> sendSms(String phone) => _post('/auth/sms/send', {'phone': phone});

  Future<Map<String, dynamic>> verifySms({
    required String phone,
    required String code,
    String? inviteCode,
    String deviceId = '',
    String platform = 'android',
  }) =>
      _post('/auth/sms/verify', {
        'phone': phone,
        'code': code,
        if (inviteCode != null) 'inviteCode': inviteCode,
        'deviceId': deviceId,
        'platform': platform,
      });

  Future<Map<String, dynamic>> pair(String code, {String deviceId = ''}) =>
      _post('/auth/pair', {'code': code, 'deviceId': deviceId, 'platform': 'android'});

  Future<Map<String, dynamic>> me() async {
    final r = await http
        .get(_u('/auth/me'), headers: _headers)
        .timeout(const Duration(seconds: 10));
    final j = jsonDecode(utf8.decode(r.bodyBytes)) as Map<String, dynamic>;
    if (r.statusCode >= 400 || j['ok'] == false) {
      throw ApiError((j['code'] ?? 'error') as String, (j['message'] ?? '未登录') as String);
    }
    return j;
  }
}

class ApiError implements Exception {
  final String code;
  final String message;
  ApiError(this.code, this.message);
  @override
  String toString() => message;
}
