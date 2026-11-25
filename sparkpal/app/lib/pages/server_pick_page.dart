import 'package:flutter/material.dart';

import '../services/local_store.dart';
import '../services/spark_api.dart';
import '../theme/app_theme.dart';
import 'gate_page.dart';

/// 首启 / 设置里选择服务器。
///
/// 官方是主按钮（默认高亮），自建是次要按钮。
/// 添加非官方地址必须先勾选风险确认。
class ServerPickPage extends StatefulWidget {
  final bool firstRun;
  const ServerPickPage({super.key, this.firstRun = true});

  @override
  State<ServerPickPage> createState() => _ServerPickPageState();
}

class _ServerPickPageState extends State<ServerPickPage> {
  List<ServerEntry> _list = const [];
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    final l = await LocalStore.servers();
    if (mounted) setState(() => _list = l);
  }

  Future<void> _use(ServerEntry e) async {
    setState(() => _busy = true);
    try {
      final meta = await SparkApi.probe(e.address);
      await LocalStore.setCurrent(e.normalized);
      if (!mounted) return;
      Navigator.of(context).pushReplacement(MaterialPageRoute(
        builder: (_) => GatePage(address: e.normalized, meta: meta),
      ));
    } on ApiError catch (err) {
      _toast(err.message);
    } catch (_) {
      _toast('连不上这台服务器，检查地址和网络');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _toast(String m) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(m)));
  }

  Future<void> _addServer() async {
    final r = await showModalBottomSheet<ServerEntry>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Sp.card,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (_) => const _AddServerSheet(),
    );
    if (r == null) return;
    await LocalStore.addServer(r);
    await _load();
    if (mounted) _use(r);
  }

  @override
  Widget build(BuildContext context) {
    final isFirst = widget.firstRun;
    return Scaffold(
      body: SafeArea(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            if (!isFirst)
              Padding(
                padding: const EdgeInsets.fromLTRB(Sp.s4, Sp.s2, Sp.s4, 0),
                child: Row(
                  children: [
                    GestureDetector(
                      onTap: () => Navigator.of(context).maybePop(),
                      child: Icon(Icons.chevron_left, size: 24, color: Sp.body),
                    ),
                    const Spacer(),
                    Text('选择服务器',
                        style: TextStyle(
                            fontSize: Sp.fsName,
                            fontWeight: FontWeight.w600,
                            color: Sp.text)),
                    const Spacer(),
                    const SizedBox(width: 24),
                  ],
                ),
              ),
            Expanded(
              child: Center(
                child: SingleChildScrollView(
                  padding: const EdgeInsets.symmetric(horizontal: Sp.s4),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      if (isFirst) ...[
                        const SizedBox(height: Sp.s8),
                        const Center(child: _Flame(size: 56)),
                        const SizedBox(height: Sp.s6),
                        Text('伙伴',
                            textAlign: TextAlign.center,
                            style: TextStyle(
                                fontSize: Sp.fsTitle,
                                fontWeight: FontWeight.w600,
                                color: Sp.text)),
                        const SizedBox(height: Sp.s2),
                        Text('让每一段火花 都不白攒',
                            textAlign: TextAlign.center,
                            style: TextStyle(fontSize: Sp.fsSub, color: Sp.sub)),
                        const SizedBox(height: Sp.s8),
                      ],
                      FilledButton(
                        onPressed: _busy ? null : () => _use(_list.isNotEmpty ? _list.first : LocalStore.officialEntry),
                        child: _busy
                            ? const SizedBox(
                                width: 18,
                                height: 18,
                                child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white))
                            : const Text('使用官方服务'),
                      ),
                      const SizedBox(height: Sp.s2),
                      Text('开箱即用，不用自己维护',
                          textAlign: TextAlign.center,
                          style: TextStyle(fontSize: Sp.fsSmall, color: Sp.sub)),
                      const SizedBox(height: Sp.s4),
                      OutlinedButton(
                        onPressed: _busy ? null : _addServer,
                        child: const Text('连接我的服务器'),
                      ),
                      const SizedBox(height: Sp.s2),
                      Text('自己部署，功能全开',
                          textAlign: TextAlign.center,
                          style: TextStyle(fontSize: Sp.fsSmall, color: Sp.sub)),
                      if (_list.length > 1) ...[
                        const SizedBox(height: Sp.s6),
                        Align(
                          alignment: Alignment.centerLeft,
                          child: Text('用过的服务器',
                              style: TextStyle(
                                  fontSize: Sp.fsSmall,
                                  fontWeight: FontWeight.w600,
                                  color: Sp.sub)),
                        ),
                        const SizedBox(height: Sp.s2),
                        for (final e in _list.where((x) => !x.official))
                          _ServerTile(
                            entry: e,
                            onTap: () => _use(e),
                            onRemove: () async {
                              await LocalStore.removeServer(e);
                              await _load();
                            },
                          ),
                      ],
                      const SizedBox(height: Sp.s6),
                      Center(
                        child: Text('自建部署文档',
                            style: TextStyle(fontSize: Sp.fsSmall, color: Sp.spark)),
                      ),
                      const SizedBox(height: Sp.s6),
                    ],
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _ServerTile extends StatelessWidget {
  final ServerEntry entry;
  final VoidCallback onTap;
  final VoidCallback onRemove;
  const _ServerTile({required this.entry, required this.onTap, required this.onRemove});

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: Sp.s2),
      decoration: BoxDecoration(
        color: Sp.card,
        borderRadius: BorderRadius.circular(Sp.rCard),
        border: Border.all(color: Sp.border),
      ),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(Sp.rCard),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: Sp.s3, vertical: Sp.s3),
          child: Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(entry.label.isEmpty ? entry.normalized : entry.label,
                        style: TextStyle(fontSize: Sp.fsItem, color: Sp.text)),
                    const SizedBox(height: Sp.s1),
                    Text(entry.normalized,
                        style: TextStyle(fontSize: Sp.fsSmall, color: Sp.sub)),
                  ],
                ),
              ),
              GestureDetector(
                onTap: onRemove,
                child: Icon(Icons.close, size: 16, color: Sp.faint),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// 添加服务器：地址 → 探测 → 第三方风险确认
class _AddServerSheet extends StatefulWidget {
  const _AddServerSheet();
  @override
  State<_AddServerSheet> createState() => _AddServerSheetState();
}

class _AddServerSheetState extends State<_AddServerSheet> {
  final _c = TextEditingController();
  bool _agree = false;
  bool _busy = false;
  String? _err;

  @override
  void dispose() {
    _c.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final addr = ServerEntry(address: _c.text, label: '');
    if (addr.normalized.isEmpty) {
      setState(() => _err = '请输入服务器地址');
      return;
    }
    if (!_agree) {
      setState(() => _err = '请先确认风险说明');
      return;
    }
    setState(() {
      _busy = true;
      _err = null;
    });
    try {
      final meta = await SparkApi.probe(addr.normalized);
      if (!mounted) return;
      Navigator.of(context).pop(ServerEntry(
        address: addr.normalized,
        label: meta.name,
        official: false,
      ));
    } on ApiError catch (e) {
      setState(() => _err = e.message);
    } catch (_) {
      setState(() => _err = '连不上这台服务器');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final bottom = MediaQuery.of(context).viewInsets.bottom;
    return Padding(
      padding: EdgeInsets.only(bottom: bottom),
      child: SingleChildScrollView(
        padding: const EdgeInsets.fromLTRB(Sp.s4, Sp.s4, Sp.s4, Sp.s6),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text('连接我的服务器',
                style: TextStyle(
                    fontSize: Sp.fsName, fontWeight: FontWeight.w600, color: Sp.text)),
            const SizedBox(height: Sp.s4),
            TextField(
              controller: _c,
              keyboardType: TextInputType.url,
              decoration: const InputDecoration(hintText: 'http://192.168.1.10:3210'),
            ),
            const SizedBox(height: Sp.s4),
            Container(
              padding: const EdgeInsets.all(Sp.s3),
              decoration: BoxDecoration(
                color: Sp.bg,
                borderRadius: BorderRadius.circular(Sp.rCard),
                border: Border.all(color: Sp.border),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('连接第三方服务器需注意',
                      style: TextStyle(
                          fontSize: Sp.fsSub,
                          fontWeight: FontWeight.w600,
                          color: Sp.text)),
                  const SizedBox(height: Sp.s2),
                  Text(
                    '这台服务器不是伙伴官方。连接后它会保存：\n'
                    '· 你的抖音登录凭证\n'
                    '· 好友列表与火花天数\n'
                    '· 你的发送记录\n\n'
                    '请确认它是你自己部署，或你完全信任。',
                    style: TextStyle(fontSize: Sp.fsSmall, height: 1.8, color: Sp.sub),
                  ),
                  const SizedBox(height: Sp.s2),
                  InkWell(
                    onTap: () => setState(() => _agree = !_agree),
                    child: Row(
                      children: [
                        _Box(checked: _agree),
                        const SizedBox(width: Sp.s2),
                        Expanded(
                          child: Text('我已了解并信任这台服务器',
                              style: TextStyle(fontSize: Sp.fsSmall, color: Sp.body)),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
            if (_err != null) ...[
              const SizedBox(height: Sp.s3),
              Text(_err!, style: TextStyle(fontSize: Sp.fsSmall, color: Sp.spark)),
            ],
            const SizedBox(height: Sp.s6),
            Row(
              children: [
                Expanded(
                  child: OutlinedButton(
                    onPressed: () => Navigator.of(context).pop(),
                    child: const Text('取消'),
                  ),
                ),
                const SizedBox(width: Sp.s2),
                Expanded(
                  flex: 13,
                  child: FilledButton(
                    onPressed: _busy || !_agree ? null : _submit,
                    child: _busy
                        ? const SizedBox(
                            width: 18,
                            height: 18,
                            child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white))
                        : const Text('继续连接'),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _Box extends StatelessWidget {
  final bool checked;
  const _Box({required this.checked});
  @override
  Widget build(BuildContext context) {
    return Container(
      width: 20,
      height: 20,
      decoration: BoxDecoration(
        color: checked ? Sp.btnBg : Sp.card,
        borderRadius: BorderRadius.circular(4),
        border: Border.all(color: checked ? Sp.btnBg : Sp.borderCtl, width: 1.4),
      ),
      child: checked
          ? Icon(Icons.check, size: 13, color: Sp.btnFg)
          : null,
    );
  }
}

class _Flame extends StatelessWidget {
  final double size;
  const _Flame({required this.size});
  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: size,
      height: size,
      child: CustomPaint(painter: _FlamePainter()),
    );
  }
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
