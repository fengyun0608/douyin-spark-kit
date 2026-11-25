import 'package:flutter/material.dart';

import '../services/local_store.dart';
import '../services/spark_api.dart';
import '../theme/app_theme.dart';

/// 首页：倒计时 + 已有火花的好友 + 今天要发的人 + 昨日记录。
///
/// 这一版是骨架：绑定抖音、好友、定时任务还没接上，
/// 数字全部来自 /api/spark/auth/me 与后续接口。
class HomePage extends StatefulWidget {
  final String address;
  final ServerMeta meta;
  const HomePage({super.key, required this.address, required this.meta});

  @override
  State<HomePage> createState() => _HomePageState();
}

class _HomePageState extends State<HomePage> {
  String _name = '';
  String? _phone;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final tok = await LocalStore.token(widget.address);
      final api = SparkApi(widget.address, token: tok);
      final me = await api.me();
      if (!mounted) return;
      setState(() {
        _name = (me['nickname'] as String?)?.trim().isNotEmpty == true
            ? me['nickname'] as String
            : (me['phone'] as String? ?? '伙伴用户');
        _phone = me['phone'] as String?;
        _loading = false;
      });
    } catch (_) {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _logout() async {
    await LocalStore.clearToken(widget.address);
    if (!mounted) return;
    Navigator.of(context).pushAndRemoveUntil(
      MaterialPageRoute(builder: (_) => const _NeedRestart()),
      (_) => false,
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: RefreshIndicator(
          onRefresh: _load,
          color: Sp.sub,
          child: ListView(
            padding: const EdgeInsets.symmetric(horizontal: Sp.s4),
            children: [
              const SizedBox(height: Sp.s2),
              Row(
                children: [
                  const _Flame(size: 20),
                  const SizedBox(width: Sp.s2),
                  Text('伙伴',
                      style: TextStyle(
                          fontSize: Sp.fsName,
                          fontWeight: FontWeight.w600,
                          color: Sp.text)),
                  const Spacer(),
                  GestureDetector(
                    onTap: _logout,
                    child: Text('退出',
                        style: TextStyle(fontSize: Sp.fsSub, color: Sp.sub)),
                  ),
                ],
              ),
              const SizedBox(height: Sp.s4),

              // 倒计时卡
              _Card(
                child: Column(
                  children: [
                    Text('距离今天自动发送',
                        style: TextStyle(fontSize: Sp.fsSmall, color: Sp.sub)),
                    const SizedBox(height: Sp.s2),
                    Text('--:--',
                        style: TextStyle(
                          fontSize: 36,
                          fontWeight: FontWeight.w700,
                          letterSpacing: 0.5,
                          color: Sp.text,
                          fontFeatures: const [FontFeature.tabularFigures()],
                        )),
                    const SizedBox(height: Sp.s2),
                    Text('还没有绑定抖音号',
                        style: TextStyle(fontSize: Sp.fsSmall, color: Sp.sub)),
                  ],
                ),
              ),
              const SizedBox(height: Sp.s2),

              // 绑定引导（还没有账号时）
              _Card(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('开始使用',
                        style: TextStyle(
                            fontSize: Sp.fsSub,
                            fontWeight: FontWeight.w600,
                            color: Sp.text)),
                    const SizedBox(height: Sp.s2),
                    Text('绑定你的抖音号，勾选要保的好友，之后每天自动发送。',
                        style: TextStyle(
                            fontSize: Sp.fsSmall, height: 1.7, color: Sp.sub)),
                    const SizedBox(height: Sp.s3),
                    FilledButton(
                      onPressed: () {
                        ScaffoldMessenger.of(context).showSnackBar(
                          const SnackBar(content: Text('扫码绑定正在开发中')),
                        );
                      },
                      child: const Text('绑定抖音号'),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: Sp.s2),

              // 已有火花的好友（占位）
              _Card(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Text('已有火花的好友',
                            style: TextStyle(
                                fontSize: Sp.fsSub,
                                fontWeight: FontWeight.w600,
                                color: Sp.text)),
                        const Spacer(),
                        Text('共 0 位',
                            style: TextStyle(
                                fontSize: Sp.fsSmall, color: Sp.sub)),
                      ],
                    ),
                    const SizedBox(height: Sp.s3),
                    Text('绑定后会按火花天数排序显示',
                        style: TextStyle(fontSize: Sp.fsSmall, color: Sp.sub)),
                  ],
                ),
              ),
              const SizedBox(height: Sp.s2),

              _Card(
                child: Row(
                  children: [
                    Text('当前服务器',
                        style: TextStyle(fontSize: Sp.fsSmall, color: Sp.sub)),
                    const Spacer(),
                    Text(widget.meta.name,
                        style: TextStyle(fontSize: Sp.fsSmall, color: Sp.body)),
                  ],
                ),
              ),
              const SizedBox(height: Sp.s6),
              Center(
                child: Text(
                  _loading ? '加载中…' : '${_name.isEmpty ? "伙伴用户" : _name}${_phone != null && _phone!.isNotEmpty ? " · ${_phone!.substring(0, 3)}****${_phone!.substring(7)}" : ""}',
                  style: TextStyle(fontSize: Sp.fsSmall, color: Sp.sub),
                ),
              ),
              const SizedBox(height: Sp.s6),
            ],
          ),
        ),
      ),
    );
  }
}

class _Card extends StatelessWidget {
  final Widget child;
  const _Card({required this.child});
  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(Sp.s4),
      decoration: BoxDecoration(
        color: Sp.card,
        borderRadius: BorderRadius.circular(Sp.rCard),
        border: Border.all(color: Sp.border),
      ),
      child: child,
    );
  }
}

class _NeedRestart extends StatelessWidget {
  const _NeedRestart();
  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Center(
        child: Text('已退出，请重新打开 App',
            style: TextStyle(fontSize: Sp.fsBody, color: Sp.sub)),
      ),
    );
  }
}

class _Flame extends StatelessWidget {
  final double size;
  const _Flame({required this.size});
  @override
  Widget build(BuildContext context) =>
      SizedBox(width: size, height: size, child: CustomPaint(painter: _FlamePainter()));
}

class _FlamePainter extends CustomPainter {
  @override
  void paint(Canvas canvas, Size size) {
    final p = Paint()
      ..shader = const LinearGradient(
        begin: Alignment.topCenter,
        end: Alignment.bottomCenter,
        colors: [Color(0xFFFFB061), Color(0xFFE5321F)],
      ).createShader(Rect.fromLTWH(0, 0, size.width, size.height))
      ..isAntiAlias = true;
    final w = size.width, h = size.height;
    final path = Path()
      ..moveTo(w * 0.60, h * 0.04)
      ..cubicTo(w * 0.62, h * 0.26, w * 0.80, h * 0.46, w * 0.80, h * 0.68)
      ..cubicTo(w * 0.80, h * 0.90, w * 0.66, h * 1.00, w * 0.50, h * 1.00)
      ..cubicTo(w * 0.34, h * 1.00, w * 0.20, h * 0.90, w * 0.20, h * 0.68)
      ..cubicTo(w * 0.20, h * 0.46, w * 0.36, h * 0.30, w * 0.44, h * 0.18)
      ..cubicTo(w * 0.41, h * 0.34, w * 0.47, h * 0.44, w * 0.53, h * 0.46)
      ..cubicTo(w * 0.57, h * 0.47, w * 0.60, h * 0.44, w * 0.59, h * 0.38)
      ..cubicTo(w * 0.58, h * 0.26, w * 0.57, h * 0.14, w * 0.60, h * 0.04)
      ..close();
    canvas.drawPath(path, p);
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}
