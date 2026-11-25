import 'package:flutter/material.dart';

import '../services/local_store.dart';
import '../services/spark_api.dart';
import '../theme/app_theme.dart';
import 'home_page.dart';

/// 自建服务器配对：输入服务器终端打印的配对码。
class PairPage extends StatefulWidget {
  final String address;
  final ServerMeta meta;
  const PairPage({super.key, required this.address, required this.meta});

  @override
  State<PairPage> createState() => _PairPageState();
}

class _PairPageState extends State<PairPage> {
  final _c = TextEditingController();
  bool _busy = false;
  String? _err;

  @override
  void dispose() {
    _c.dispose();
    super.dispose();
  }

  Future<void> _pair() async {
    final code = _c.text.trim();
    if (code.isEmpty) {
      setState(() => _err = '请输入配对码');
      return;
    }
    setState(() {
      _busy = true;
      _err = null;
    });
    try {
      final api = SparkApi(widget.address);
      final r = await api.pair(code);
      final token = r['token'] as String?;
      if (token == null) throw ApiError('no_token', '配对返回异常');
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
      setState(() => _err = '配对失败，稍后再试');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('连接我的服务器')),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: Sp.s4),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const SizedBox(height: Sp.s6),
              Text('输入配对码',
                  style: TextStyle(
                      fontSize: Sp.fsTitle,
                      fontWeight: FontWeight.w600,
                      color: Sp.text)),
              const SizedBox(height: Sp.s2),
              Text('服务器启动时终端会打印 6 位配对码',
                  style: TextStyle(fontSize: Sp.fsSmall, color: Sp.sub)),
              const SizedBox(height: Sp.s8),
              TextField(
                controller: _c,
                keyboardType: TextInputType.number,
                maxLength: 6,
                textAlign: TextAlign.center,
                style: TextStyle(
                  fontSize: 22,
                  fontWeight: FontWeight.w600,
                  letterSpacing: 8,
                  color: Sp.text,
                ),
                decoration: const InputDecoration(counterText: ''),
                onSubmitted: (_) => _pair(),
              ),
              if (_err != null) ...[
                const SizedBox(height: Sp.s3),
                Text(_err!, style: TextStyle(fontSize: Sp.fsSmall, color: Sp.spark)),
              ],
              const SizedBox(height: Sp.s6),
              FilledButton(
                onPressed: _busy ? null : _pair,
                child: _busy
                    ? const SizedBox(
                        width: 18,
                        height: 18,
                        child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white))
                    : const Text('配对'),
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
