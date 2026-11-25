import 'package:flutter/material.dart';

import '../services/local_store.dart';
import '../services/spark_api.dart';
import '../theme/app_theme.dart';
import 'home_page.dart';
import 'invite_page.dart';
import 'login_page.dart';
import 'pair_page.dart';

/// 选完服务器之后的分流：
/// 已有 token → 首页
/// 需要邀请码 → 邀请码页
/// 需要配对码 → 配对页
/// 否则 → 手机号登录
class GatePage extends StatefulWidget {
  final String address;
  final ServerMeta meta;
  const GatePage({super.key, required this.address, required this.meta});

  @override
  State<GatePage> createState() => _GatePageState();
}

class _GatePageState extends State<GatePage> {
  bool _checking = true;

  @override
  void initState() {
    super.initState();
    _route();
  }

  Future<void> _route() async {
    final tok = await LocalStore.token(widget.address);
    if (!mounted) return;

    if (tok != null && tok.isNotEmpty) {
      try {
        await SparkApi(widget.address, token: tok).me();
        if (!mounted) return;
        _go(HomePage(address: widget.address, meta: widget.meta));
        return;
      } catch (_) {
        await LocalStore.clearToken(widget.address);
      }
    }

    if (!mounted) return;
    if (widget.meta.needPairCode) {
      _go(PairPage(address: widget.address, meta: widget.meta));
    } else if (widget.meta.needInvite) {
      _go(InvitePage(address: widget.address, meta: widget.meta));
    } else {
      _go(LoginPage(address: widget.address, meta: widget.meta));
    }
  }

  void _go(Widget page) {
    setState(() => _checking = false);
    Navigator.of(context).pushReplacement(MaterialPageRoute(builder: (_) => page));
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Center(
        child: _checking
            ? SizedBox(
                width: 22,
                height: 22,
                child: CircularProgressIndicator(strokeWidth: 2, color: Sp.sub),
              )
            : const SizedBox.shrink(),
      ),
    );
  }
}
