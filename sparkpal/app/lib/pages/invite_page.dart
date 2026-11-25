import 'package:flutter/material.dart';

import '../services/spark_api.dart';
import '../theme/app_theme.dart';
import 'login_page.dart';

/// 邀请码：内测阶段挡在登录前面。
class InvitePage extends StatefulWidget {
  final String address;
  final ServerMeta meta;
  const InvitePage({super.key, required this.address, required this.meta});

  @override
  State<InvitePage> createState() => _InvitePageState();
}

class _InvitePageState extends State<InvitePage> {
  final _c = TextEditingController();
  bool _busy = false;
  String? _err;

  @override
  void dispose() {
    _c.dispose();
    super.dispose();
  }

  Future<void> _next() async {
    final code = _c.text.trim().toUpperCase();
    if (code.isEmpty) {
      setState(() => _err = '请输入邀请码');
      return;
    }
    setState(() {
      _busy = true;
      _err = null;
    });
    try {
      await SparkApi(widget.address).checkInvite(code);
      if (!mounted) return;
      Navigator.of(context).pushReplacement(MaterialPageRoute(
        builder: (_) => LoginPage(
          address: widget.address,
          meta: widget.meta,
          inviteCode: code,
        ),
      ));
    } on ApiError catch (e) {
      setState(() => _err = e.message);
    } catch (_) {
      setState(() => _err = '网络异常，稍后再试');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('邀请码')),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: Sp.s4),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const SizedBox(height: Sp.s4),
              Text('内测阶段需要邀请码',
                  style: TextStyle(fontSize: Sp.fsBody, color: Sp.body)),
              const SizedBox(height: Sp.s2),
              Text('每个邀请码可绑定 3 个账号',
                  style: TextStyle(fontSize: Sp.fsSmall, color: Sp.sub)),
              const SizedBox(height: Sp.s6),
              TextField(
                controller: _c,
                textCapitalization: TextCapitalization.characters,
                textAlign: TextAlign.center,
                style: TextStyle(
                  fontSize: 20,
                  fontWeight: FontWeight.w600,
                  letterSpacing: 6,
                  color: Sp.text,
                ),
                decoration: const InputDecoration(hintText: '输入邀请码'),
                onSubmitted: (_) => _next(),
              ),
              if (_err != null) ...[
                const SizedBox(height: Sp.s3),
                Text(_err!, style: TextStyle(fontSize: Sp.fsSmall, color: Sp.spark)),
              ],
              const SizedBox(height: Sp.s6),
              FilledButton(
                onPressed: _busy ? null : _next,
                child: _busy
                    ? const SizedBox(
                        width: 18,
                        height: 18,
                        child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white))
                    : const Text('继续'),
              ),
              const SizedBox(height: Sp.s4),
              Center(
                child: Text('还没有邀请码？关注更新通知',
                    style: TextStyle(fontSize: Sp.fsSmall, color: Sp.sub)),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
