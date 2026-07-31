import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../models/media_item.dart';

class DuplicatesPage extends StatefulWidget {
  const DuplicatesPage({super.key});

  @override
  State<DuplicatesPage> createState() => _DuplicatesPageState();
}

class _DuplicatesPageState extends State<DuplicatesPage>
    with AutomaticKeepAliveClientMixin {
  final _api = ApiService();
  bool _aiEnabled = false;
  bool _loading = true;
  List<DuplicateGroup> _duplicateGroups = [];
  String? _error;

  @override
  bool get wantKeepAlive => true;

  @override
  void initState() {
    super.initState();
    _checkAndLoad();
  }

  Future<void> _checkAndLoad() async {
    setState(() {
      _loading = true;
      _error = null;
    });

    final settingsRes = await _api.getSettings();
    if (!mounted) return;

    if (settingsRes.isSuccess) {
      _aiEnabled = settingsRes.data?['ai_enabled'] == true;
      if (_aiEnabled) {
        final dupRes = await _api.getDuplicates();
        if (!mounted) return;
        if (dupRes.isSuccess) {
          setState(() {
            _duplicateGroups = dupRes.data ?? [];
            _loading = false;
          });
        } else {
          setState(() {
            _error = dupRes.message ?? '获取查重数据失败';
            _loading = false;
          });
        }
      } else {
        setState(() {
          _loading = false;
        });
      }
    } else {
      setState(() {
        _error = settingsRes.message ?? '获取设置失败';
        _loading = false;
      });
    }
  }

  Future<void> _deleteItem(MediaItem item, int groupIdx) async {
    final name = item.name;
    final path = item.path;
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('确认删除'),
        content: Text('确定要删除相似图片 "$name" 吗？此操作不可撤销。'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('取消'),
          ),
          FilledButton(
            style: FilledButton.styleFrom(
              backgroundColor: Theme.of(ctx).colorScheme.error,
            ),
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('删除'),
          ),
        ],
      ),
    );
    if (confirm != true) return;

    setState(() => _loading = true);
    final res = await _api.deleteFile(path);
    if (!mounted) return;

    if (res.isSuccess) {
      setState(() {
        final group = _duplicateGroups[groupIdx];
        final items = List<MediaItem>.from(group.items);
        items.removeWhere((i) => i.path == path);

        if (items.length < 2) {
          _duplicateGroups.removeAt(groupIdx);
        } else {
          _duplicateGroups[groupIdx] = DuplicateGroup(
            similarity: group.similarity,
            items: items,
          );
        }
        _loading = false;
      });
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('删除成功')));
    } else {
      setState(() => _loading = false);
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(res.message ?? '删除失败')));
    }
  }

  String _formatBytes(dynamic value) {
    final bytes = (value as num?)?.toInt() ?? 0;
    if (bytes < 1024) return '$bytes B';
    if (bytes < 1024 * 1024) {
      return '${(bytes / 1024).toStringAsFixed(1)} KB';
    }
    if (bytes < 1024 * 1024 * 1024) {
      return '${(bytes / (1024 * 1024)).toStringAsFixed(1)} MB';
    }
    return '${(bytes / (1024 * 1024 * 1024)).toStringAsFixed(1)} GB';
  }

  void _previewImage(String path, String name) {
    showDialog(
      context: context,
      builder: (ctx) => GestureDetector(
        onTap: () => Navigator.pop(ctx),
        child: Dialog(
          backgroundColor: Colors.black,
          insetPadding: EdgeInsets.zero,
          child: Stack(
            fit: StackFit.expand,
            children: [
              InteractiveViewer(
                child: Image.network(
                  _api.getDownloadUrl(path, disposition: 'inline'),
                  fit: BoxFit.contain,
                ),
              ),
              Positioned(
                top: 40,
                right: 20,
                child: CircleAvatar(
                  backgroundColor: Colors.black54,
                  child: IconButton(
                    icon: const Icon(Icons.close, color: Colors.white),
                    onPressed: () => Navigator.pop(ctx),
                  ),
                ),
              ),
              Positioned(
                bottom: 20,
                left: 20,
                right: 20,
                child: Text(
                  name,
                  style: const TextStyle(color: Colors.white, fontSize: 14),
                  textAlign: TextAlign.center,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    super.build(context);
    final theme = Theme.of(context);

    Widget body;

    if (_loading) {
      body = const Center(child: CircularProgressIndicator());
    } else if (_error != null) {
      body = Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.error_outline, size: 48, color: theme.colorScheme.error),
            const SizedBox(height: 12),
            Text(_error!),
            const SizedBox(height: 12),
            FilledButton.tonal(
              onPressed: _checkAndLoad,
              child: const Text('重试'),
            ),
          ],
        ),
      );
    } else if (!_aiEnabled) {
      body = Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(Icons.psychology_alt, size: 80, color: Colors.grey.shade300),
              const SizedBox(height: 16),
              const Text(
                'AI 查重功能未开启',
                style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
              ),
              const SizedBox(height: 8),
              Text(
                '智能查重需要基于多模态向量特征计算，请在“设置”页开启“启用 AI 搜索与图片查重”',
                textAlign: TextAlign.center,
                style: TextStyle(color: Colors.grey.shade600),
              ),
            ],
          ),
        ),
      );
    } else if (_duplicateGroups.isEmpty) {
      body = Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.check_circle_outline,
              size: 80,
              color: Colors.green.shade200,
            ),
            const SizedBox(height: 16),
            const Text(
              '没有发现相似图片',
              style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 8),
            Text('您的相册非常整洁！', style: TextStyle(color: Colors.grey.shade600)),
          ],
        ),
      );
    } else {
      body = RefreshIndicator(
        onRefresh: _checkAndLoad,
        child: ListView.builder(
          padding: const EdgeInsets.all(12),
          itemCount: _duplicateGroups.length,
          itemBuilder: (context, idx) {
            final group = _duplicateGroups[idx];
            final similarity = group.similarity;
            final items = group.items;

            return Card(
              margin: const EdgeInsets.only(bottom: 16),
              color: theme.colorScheme.surfaceContainerHighest.withValues(
                alpha: 0.3,
              ),
              child: Padding(
                padding: const EdgeInsets.all(12),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 8,
                            vertical: 4,
                          ),
                          decoration: BoxDecoration(
                            color: theme.colorScheme.primary,
                            borderRadius: BorderRadius.circular(4),
                          ),
                          child: Text(
                            '相似度 ${(similarity * 100).toStringAsFixed(1)}%',
                            style: const TextStyle(
                              color: Colors.white,
                              fontSize: 12,
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                        ),
                        const SizedBox(width: 8),
                        Text(
                          '发现 ${items.length} 张高度相似照片',
                          style: theme.textTheme.bodyMedium?.copyWith(
                            color: theme.colorScheme.onSurfaceVariant,
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 12),
                    SizedBox(
                      height: 170,
                      child: ListView.builder(
                        scrollDirection: Axis.horizontal,
                        itemCount: items.length,
                        itemBuilder: (context, itemIdx) {
                          final item = items[itemIdx];
                          final name = item.name;
                          final path = item.path;
                          final size = item.size;

                          return Container(
                            width: 120,
                            margin: const EdgeInsets.only(right: 12),
                            child: Stack(
                              children: [
                                Column(
                                  children: [
                                    Expanded(
                                      child: ClipRRect(
                                        borderRadius: BorderRadius.circular(8),
                                        child: GestureDetector(
                                          onTap: () =>
                                              _previewImage(path, name),
                                          child: Image.network(
                                            _api.getThumbUrl(path),
                                            fit: BoxFit.cover,
                                            width: 120,
                                            errorBuilder: (_, _, _) =>
                                                Container(
                                                  color: Colors.grey.shade200,
                                                  child: const Icon(
                                                    Icons.image,
                                                  ),
                                                ),
                                          ),
                                        ),
                                      ),
                                    ),
                                    const SizedBox(height: 4),
                                    Text(
                                      name,
                                      maxLines: 1,
                                      overflow: TextOverflow.ellipsis,
                                      style: const TextStyle(fontSize: 11),
                                      textAlign: TextAlign.center,
                                    ),
                                    const SizedBox(height: 2),
                                    Text(
                                      _formatBytes(size),
                                      style: TextStyle(
                                        fontSize: 10,
                                        color:
                                            theme.colorScheme.onSurfaceVariant,
                                      ),
                                    ),
                                  ],
                                ),
                                Positioned(
                                  top: 4,
                                  right: 4,
                                  child: CircleAvatar(
                                    radius: 14,
                                    backgroundColor: Colors.black54,
                                    child: IconButton(
                                      padding: EdgeInsets.zero,
                                      constraints: const BoxConstraints(),
                                      icon: const Icon(
                                        Icons.delete,
                                        size: 14,
                                        color: Colors.red,
                                      ),
                                      onPressed: () => _deleteItem(item, idx),
                                    ),
                                  ),
                                ),
                              ],
                            ),
                          );
                        },
                      ),
                    ),
                  ],
                ),
              ),
            );
          },
        ),
      );
    }

    return Scaffold(
      appBar: AppBar(
        title: const Text('图片查重'),
        actions: [
          IconButton(icon: const Icon(Icons.refresh), onPressed: _checkAndLoad),
        ],
      ),
      body: body,
    );
  }
}
