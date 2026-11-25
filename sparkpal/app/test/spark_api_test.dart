import 'package:flutter_test/flutter_test.dart';
import 'package:sparkpal/services/spark_api.dart';

void main() {
  group('ServerEntry.normalized', () {
    test('补协议头', () {
      final e = ServerEntry(address: '192.168.1.10:3210', label: '');
      expect(e.normalized, 'http://192.168.1.10:3210');
    });

    test('保留 https', () {
      final e = ServerEntry(address: 'https://114.66.28.185:48285', label: '');
      expect(e.normalized, 'https://114.66.28.185:48285');
    });

    test('去掉结尾斜杠', () {
      final e = ServerEntry(address: 'http://a.com/', label: '');
      expect(e.normalized, 'http://a.com');
    });

    test('空地址', () {
      final e = ServerEntry(address: '   ', label: '');
      expect(e.normalized, '');
    });
  });

  group('ServerMeta.fromJson', () {
    test('官方 + 免费期', () {
      final m = ServerMeta.fromJson({
        'app': 'sparkpal',
        'apiVersion': 1,
        'name': '伙伴官方',
        'official': true,
        'billing': false,
        'needPairCode': false,
        'needInvite': true,
        'quota': {'plan': 'unlimited', 'maxAccounts': -1, 'maxFriends': -1, 'maxDaily': -1},
      });
      expect(m.official, isTrue);
      expect(m.billing, isFalse);
      expect(m.needInvite, isTrue);
      expect(m.maxAccounts, -1);
    });

    test('自建：无邀请码无计费', () {
      final m = ServerMeta.fromJson({
        'apiVersion': 1,
        'official': false,
        'billing': false,
        'needPairCode': true,
        'needInvite': false,
      });
      expect(m.official, isFalse);
      expect(m.needPairCode, isTrue);
      expect(m.needInvite, isFalse);
      expect(m.maxDaily, 5); // 缺省值
    });
  });
}
