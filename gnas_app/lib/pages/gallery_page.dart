import 'package:flutter/material.dart';
import 'package:video_player/video_player.dart';
import 'package:wechat_assets_picker/wechat_assets_picker.dart';
import '../services/api_service.dart';
import '../models/media_item.dart';

class GalleryPage extends StatefulWidget {
  const GalleryPage({super.key});

  @override
  State<GalleryPage> createState() => _GalleryPageState();
}

class _GalleryPageState extends State<GalleryPage>
    with AutomaticKeepAliveClientMixin {
  final _api = ApiService();
  List<MediaItem> _items = [];
  bool _loading = true;
  String? _error;

  @override
  bool get wantKeepAlive => true;

  @override
  void initState() {
    super.initState();
    _loadGallery();
  }

  Future<void> _loadGallery() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    final res = await _api.getGallery();
    if (!mounted) return;
    if (res.isSuccess && res.data != null) {
      setState(() {
        _items = res.data!;
        _loading = false;
      });
    } else {
      setState(() {
        _error = res.message ?? '加载失败';
        _loading = false;
      });
    }
  }

  Future<void> _uploadMedia() async {
    final assets = await AssetPicker.pickAssets(
      context,
      pickerConfig: const AssetPickerConfig(
        maxAssets: 99,
        requestType: RequestType.common,
      ),
    );
    if (assets == null || assets.isEmpty) return;

    if (!mounted) return;
    final messenger = ScaffoldMessenger.of(context);
    messenger.showSnackBar(const SnackBar(content: Text('正在上传...')));

    int success = 0;
    int failed = 0;
    for (final asset in assets) {
      final file = await asset.originFile;
      if (file == null) {
        failed++;
        continue;
      }
      final res = await _api.uploadFile('/', file);
      if (res.isSuccess) {
        success++;
      } else {
        failed++;
      }
    }

    if (!mounted) return;
    if (failed == 0) {
      messenger.showSnackBar(
        SnackBar(
          content: Text('上传完成，成功 $success 个'),
          backgroundColor: Colors.green.shade600,
        ),
      );
    } else {
      messenger.showSnackBar(
        SnackBar(
          content: Text('成功 $success 个，失败 $failed 个'),
          backgroundColor: Colors.orange.shade600,
        ),
      );
    }
    _loadGallery();
  }

  @override
  Widget build(BuildContext context) {
    super.build(context);
    final theme = Theme.of(context);

    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }

    if (_error != null) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.error_outline, size: 48, color: theme.colorScheme.error),
            const SizedBox(height: 12),
            Text(_error!),
            const SizedBox(height: 12),
            FilledButton.tonal(
              onPressed: _loadGallery,
              child: const Text('重试'),
            ),
          ],
        ),
      );
    }

    if (_items.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.photo_library_outlined,
              size: 80,
              color: Colors.grey.shade300,
            ),
            const SizedBox(height: 12),
            Text('暂无媒体文件', style: TextStyle(color: Colors.grey.shade500)),
          ],
        ),
      );
    }

    return Stack(
      children: [
        RefreshIndicator(
          onRefresh: _loadGallery,
          child: LayoutBuilder(
            builder: (_, constraints) {
              final crossAxisCount = constraints.maxWidth > 600 ? 4 : 3;
              return GridView.builder(
                padding: const EdgeInsets.all(4),
                gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
                  crossAxisCount: crossAxisCount,
                  crossAxisSpacing: 2,
                  mainAxisSpacing: 2,
                ),
                itemCount: _items.length,
                itemBuilder: (_, i) {
                  final item = _items[i];
                  return GestureDetector(
                    onTap: () => _preview(context, item),
                    onLongPress: () => _deleteItem(item),
                    child: Stack(
                      fit: StackFit.expand,
                      children: [
                        // Try thumbnail for both images and videos
                        Image.network(
                          _api.getThumbUrl(item.path),
                          fit: BoxFit.cover,
                          errorBuilder: (_, _, _) => _placeholder(item),
                        ),
                        if (item.isVideo)
                          const Center(
                            child: SizedBox(
                              width: 48,
                              height: 48,
                              child: DecoratedBox(
                                decoration: BoxDecoration(
                                  color: Colors.black54,
                                  shape: BoxShape.circle,
                                ),
                                child: Icon(
                                  Icons.play_arrow,
                                  color: Colors.white,
                                  size: 32,
                                ),
                              ),
                            ),
                          ),
                      ],
                    ),
                  );
                },
              );
            },
          ),
        ),
        // Upload button at bottom-right
        Positioned(
          bottom: 16,
          right: 16,
          child: FloatingActionButton.small(
            heroTag: 'gallery_upload',
            onPressed: _uploadMedia,
            tooltip: '上传媒体文件',
            child: const Icon(Icons.upload),
          ),
        ),
      ],
    );
  }

  Widget _placeholder(MediaItem item) {
    return Container(
      color: Colors.grey.shade200,
      child: Icon(
        item.isImage ? Icons.image : Icons.videocam,
        color: Colors.grey.shade400,
        size: 40,
      ),
    );
  }

  Future<void> _deleteItem(MediaItem item) async {
    final messenger = ScaffoldMessenger.of(context);
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) {
        final dialogTheme = Theme.of(ctx);
        return AlertDialog(
          title: const Text('确认删除'),
          content: Text('确定要删除 "${item.name}" 吗？'),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('取消'),
            ),
            FilledButton(
              style: FilledButton.styleFrom(
                backgroundColor: dialogTheme.colorScheme.error,
              ),
              onPressed: () => Navigator.pop(ctx, true),
              child: const Text('删除'),
            ),
          ],
        );
      },
    );
    if (confirm != true) return;

    final res = await _api.deleteFile(item.path);
    if (!mounted) return;
    if (res.isSuccess) {
      messenger.showSnackBar(
        SnackBar(
          content: Text('已删除 "${item.name}"'),
          backgroundColor: Colors.green.shade600,
        ),
      );
      _loadGallery();
    } else {
      messenger.showSnackBar(
        SnackBar(
          content: Text(res.message ?? '删除失败'),
          backgroundColor: Colors.red.shade600,
        ),
      );
    }
  }

  void _preview(BuildContext context, MediaItem item) {
    if (item.isVideo) {
      Navigator.of(context).push(
        MaterialPageRoute(
          builder: (_) => _VideoPreviewPage(item: item, api: _api),
        ),
      );
    } else {
      final images = _items.where((m) => m.isImage).toList();
      final initialIndex = images.indexWhere((m) => m.path == item.path);
      if (initialIndex < 0) return;

      Navigator.of(context).push(
        MaterialPageRoute(
          builder: (_) => _ImagePreviewPage(
            images: images,
            initialIndex: initialIndex,
            api: _api,
          ),
        ),
      );
    }
  }
}

// --- Image preview (unchanged) ---

class _ImagePreviewPage extends StatefulWidget {
  final List<MediaItem> images;
  final int initialIndex;
  final ApiService api;

  const _ImagePreviewPage({
    required this.images,
    required this.initialIndex,
    required this.api,
  });

  @override
  State<_ImagePreviewPage> createState() => _ImagePreviewPageState();
}

class _ImagePreviewPageState extends State<_ImagePreviewPage> {
  late final PageController _pageController;
  late int _currentIndex;

  @override
  void initState() {
    super.initState();
    _currentIndex = widget.initialIndex;
    _pageController = PageController(initialPage: _currentIndex);
  }

  @override
  void dispose() {
    _pageController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        foregroundColor: Colors.white,
        title: Text(
          '${_currentIndex + 1} / ${widget.images.length}',
          style: const TextStyle(color: Colors.white),
        ),
        iconTheme: const IconThemeData(color: Colors.white),
      ),
      body: PageView.builder(
        controller: _pageController,
        itemCount: widget.images.length,
        onPageChanged: (i) => setState(() => _currentIndex = i),
        itemBuilder: (_, i) {
          final item = widget.images[i];
          final url = widget.api.getDownloadUrl(
            item.path,
            disposition: 'inline',
          );
          return InteractiveViewer(
            minScale: 1.0,
            maxScale: 4.0,
            child: Center(
              child: Image.network(
                url,
                fit: BoxFit.contain,
                loadingBuilder: (_, child, progress) {
                  if (progress == null) return child;
                  return Center(
                    child: CircularProgressIndicator(
                      value: progress.expectedTotalBytes != null
                          ? progress.cumulativeBytesLoaded /
                                progress.expectedTotalBytes!
                          : null,
                      color: Colors.white54,
                    ),
                  );
                },
                errorBuilder: (_, _, _) => Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    const Icon(
                      Icons.broken_image,
                      color: Colors.white38,
                      size: 64,
                    ),
                    const SizedBox(height: 8),
                    Text(
                      item.name,
                      style: const TextStyle(color: Colors.white38),
                      textAlign: TextAlign.center,
                    ),
                  ],
                ),
              ),
            ),
          );
        },
      ),
    );
  }
}

// --- Video preview ---

class _VideoPreviewPage extends StatefulWidget {
  final MediaItem item;
  final ApiService api;

  const _VideoPreviewPage({required this.item, required this.api});

  @override
  State<_VideoPreviewPage> createState() => _VideoPreviewPageState();
}

class _VideoPreviewPageState extends State<_VideoPreviewPage> {
  VideoPlayerController? _controller;
  bool _initialized = false;
  bool _error = false;
  bool _showControls = true;
  bool _listenerAttached = false;

  @override
  void initState() {
    super.initState();
    _initPlayer();
  }

  Future<void> _initPlayer() async {
    VideoPlayerController? controller;
    try {
      final url = widget.api.getDownloadUrl(
        widget.item.path,
        disposition: 'inline',
      );
      controller = VideoPlayerController.networkUrl(Uri.parse(url));
      _controller = controller;
      await controller.initialize();
      if (!mounted) {
        await controller.dispose();
        if (_controller == controller) {
          _controller = null;
        }
        return;
      }
      setState(() => _initialized = true);
      controller.play();
      controller.addListener(_onStateChanged);
      _listenerAttached = true;
    } catch (e) {
      if (controller != null) {
        await controller.dispose();
      }
      if (_controller == controller) {
        _controller = null;
      }
      if (!mounted) return;
      setState(() => _error = true);
    }
  }

  void _onStateChanged() {
    if (!mounted) return;
    setState(() {});
  }

  @override
  void dispose() {
    final controller = _controller;
    if (controller != null) {
      if (_listenerAttached) {
        controller.removeListener(_onStateChanged);
      }
      controller.dispose();
    }
    super.dispose();
  }

  String _formatDuration(Duration d) {
    final minutes = d.inMinutes.remainder(60).toString().padLeft(2, '0');
    final seconds = d.inSeconds.remainder(60).toString().padLeft(2, '0');
    return '${d.inHours > 0 ? '${d.inHours}:' : ''}$minutes:$seconds';
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        foregroundColor: Colors.white,
        title: Text(
          widget.item.name,
          style: const TextStyle(color: Colors.white, fontSize: 16),
        ),
        iconTheme: const IconThemeData(color: Colors.white),
      ),
      body: GestureDetector(
        onTap: () => setState(() => _showControls = !_showControls),
        child: _error
            ? Center(
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    const Icon(
                      Icons.videocam_off,
                      color: Colors.white38,
                      size: 64,
                    ),
                    const SizedBox(height: 12),
                    const Text(
                      '视频加载失败',
                      style: TextStyle(color: Colors.white54, fontSize: 16),
                    ),
                    const SizedBox(height: 12),
                    Text(
                      widget.item.name,
                      style: const TextStyle(color: Colors.white38),
                    ),
                    const SizedBox(height: 24),
                    FilledButton.tonal(
                      onPressed: () {
                        setState(() {
                          _error = false;
                          _initialized = false;
                        });
                        _initPlayer();
                      },
                      child: const Text('重试'),
                    ),
                  ],
                ),
              )
            : _initialized && _controller != null
            ? Stack(
                fit: StackFit.expand,
                children: [
                  Center(
                    child: AspectRatio(
                      aspectRatio: _controller!.value.aspectRatio,
                      child: VideoPlayer(_controller!),
                    ),
                  ),
                  if (_showControls)
                    Positioned(
                      left: 0,
                      right: 0,
                      bottom: 0,
                      child: Container(
                        decoration: const BoxDecoration(
                          gradient: LinearGradient(
                            begin: Alignment.topCenter,
                            end: Alignment.bottomCenter,
                            colors: [Colors.transparent, Colors.black54],
                          ),
                        ),
                        child: Padding(
                          padding: EdgeInsets.only(
                            bottom: MediaQuery.of(context).padding.bottom,
                          ),
                          child: Column(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              VideoProgressIndicator(
                                _controller!,
                                allowScrubbing: true,
                                colors: const VideoProgressColors(
                                  playedColor: Colors.white,
                                  bufferedColor: Colors.white24,
                                  backgroundColor: Colors.white10,
                                ),
                              ),
                              Padding(
                                padding: const EdgeInsets.symmetric(
                                  horizontal: 16,
                                ),
                                child: Row(
                                  mainAxisAlignment:
                                      MainAxisAlignment.spaceBetween,
                                  children: [
                                    Text(
                                      _formatDuration(
                                        _controller!.value.position,
                                      ),
                                      style: const TextStyle(
                                        color: Colors.white70,
                                        fontSize: 12,
                                      ),
                                    ),
                                    IconButton(
                                      icon: Icon(
                                        _controller!.value.isPlaying
                                            ? Icons.pause_circle_filled
                                            : Icons.play_circle_filled,
                                        color: Colors.white,
                                        size: 40,
                                      ),
                                      onPressed: () {
                                        if (_controller!.value.isPlaying) {
                                          _controller!.pause();
                                        } else {
                                          _controller!.play();
                                        }
                                      },
                                    ),
                                    Text(
                                      _formatDuration(
                                        _controller!.value.duration,
                                      ),
                                      style: const TextStyle(
                                        color: Colors.white70,
                                        fontSize: 12,
                                      ),
                                    ),
                                  ],
                                ),
                              ),
                              const SizedBox(height: 8),
                            ],
                          ),
                        ),
                      ),
                    ),
                ],
              )
            : const Center(
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    CircularProgressIndicator(color: Colors.white54),
                    SizedBox(height: 16),
                    Text('正在加载视频...', style: TextStyle(color: Colors.white54)),
                  ],
                ),
              ),
      ),
    );
  }
}
