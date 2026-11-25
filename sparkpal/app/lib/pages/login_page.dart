import 'dart:async';

import 'package:flutter/material.dart';

import '../services/local_store.dart';
import '../services/spark_api.dart';
import '../theme/app_theme.dart';
import 'home_page.dart';

/// 手机号 + 验证码登录（官方）。
class LoginPage extends StatefulWidget {
  final String address;
  final ServerMeta meta;
  final String? inviteCode;
  const LoginPage({
    super.key,
    required this.address,
    required this.meta,
    this.inviteCode,
  });

  @override
  State<LoginPage> createState() => _LoginPageState();
}

class _LoginPageState extends State<LoginPage> {
  final _phone = TextEditingController();
  final _code = TextEditingController();
  bool _busy = false;
  bool _sent = false;
  String? _err;
  Timer? _timer;
  int _left = 0;

  @override
  void dispose() {
    _timer?.cancel();
    _phone.dispose();
    _code.dispose();
    super.dispose();
  }

  void _startCountdown() {
    _timer?.cancel();
    setState(() => _left = 60);
    _timer = Timer.periodic(const Duration(seconds: 1), (t) {
      if (!mounted) return t.cancel();
      setState(() => _left--);
      if (_left <= 0) t.cancel();
    });
  }

  Future<void> _send() async {
    final p = _phone.text.trim();
    if (p.length != 11) {
      setState(() => _err = '手机号格式不对');
      return;
    }
    setState(() {
      _busy = true;
      _err = null;
    });
    try {
      await SparkApi(widget.address).sendSms(p);
      if (!mounted) return;
      setState(() => _sent = true);
      _startCountdown();
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('验证码已发送')),
      );
    } on ApiError catch (e) {
      setState(() => _err = e.message);
    } catch (_) {
      setState(() => _err = '网络异常，稍后再试');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _login() async {
    final p = _phone.text.trim();
    final c = _code.text.trim();
    if (c.length < 4) {
      setState(() => _err = '请输入验证码');
      return;
    }
    setState(() {
      _busy = true;
      _err = null;
    });
    try {
      final api = SparkApi(widget.address);
      final r = await api.verifySms(
        phone: p,
        code: c,
        inviteCode: widget.inviteCode,
      );
      final token = r['token'] as String?;
      if (token == null) throw ApiError('no_token', '登录返回异常');
      await LocalStore.setToken(widget.address, token);
      if (!mounted) return;
      Navigator.of(context).pushAndRemoveUntil(
        MaterialPageRoute(
            builder: (_) => HomePage(address: widget.address, meta: widget.meta)),
        (_) => false,
      );
    } on ApiError catch (e) {
      setState(() => _err = e.message);
    } catch (_) {
      setState(() => _err = '登录失败，稍后再试');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('登录')),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: Sp.s4),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const SizedBox(height: Sp.s6),
              Text('用手机号登录',
                  style: TextStyle(
                      fontSize: Sp.fsTitle,
                      fontWeight: FontWeight.w600,
                      color: Sp.text)),
              const SizedBox(height: Sp.s2),
              Text('账号只用于订阅、多设备同步与数据归属',
                  style: TextStyle(fontSize: Sp.fsSmall, color: Sp.sub)),
              const SizedBox(height: Sp.s8),
              TextField(
                controller: _phone,
                keyboardType: TextInputType.phone,
                maxLength: 11,
                decoration: const InputDecoration(
                  hintText: '手机号',
                  counterText: '',
                ),
              ),
              const SizedBox(height: Sp.s3),
              Row(
                children: [
                  Expanded(
                    child: TextField(
                      controller: _code,
                      keyboardType: TextInputType.number,
                      maxLength: 6,
                      decoration: const InputDecoration(
                        hintText: '验证码',
                        counterText: '',
                      ),
                    ),
                  ),
                  const SizedBox(width: Sp.s2),
                  SizedBox(
                    width: 108,
                    height: 48,
                    child: OutlinedButton(
                      onPressed: (_busy || _left > 0) ? null : _send,
                      child: Text(_left > 0 ? '${_left}s' : (_sent ? '重发' : '获取')),
                    ),
                  ),
                ],
              ),
              if (_err != null) ...[
                const SizedBox(height: Sp.s3),
                Text(_err!, style: TextStyle(fontSize: Sp.fsSmall, color: Sp.spark)),
              ],
              const SizedBox(height: Sp.s6),
              FilledButton(
                onPressed: _busy ? null : _login,
                child: _busy
                    ? const SizedBox(
                        width: 18,
                        height: 18,
                        child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white))
                    : const Text('登录'),
              ),
              const Spacer(),
              Center(
                child: Text('当前服务器：${widget.meta.name}',
                    style: TextStyle(fontSize: Sp.fsSmall, color: Sp.sub)),
              ),
              const SizedBox(height: Sp.s6),
            ],
          ),
        ),
      ),
    );
  }
}
