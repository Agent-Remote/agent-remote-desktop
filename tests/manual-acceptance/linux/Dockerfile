# Disposable Linux desktop for agent-remote-desktop hands-on acceptance:
# X11 (Xvfb) + openbox, shown over VNC; fcitx5 Pinyin and Orca for the
# input-method and screen-reader checks. Nothing persists.
FROM debian:bookworm-slim
ENV DEBIAN_FRONTEND=noninteractive LANG=C.UTF-8
RUN apt-get update -qq && apt-get install -y -qq --no-install-recommends \
      libgtk-3-0 libayatana-appindicator3-1 libpango-1.0-0 libpangocairo-1.0-0 \
      xvfb x11vnc openbox xterm xdotool dbus-x11 ca-certificates \
      fonts-noto-cjk fonts-dejavu-core \
      fcitx5 fcitx5-chinese-addons fcitx5-frontend-gtk3 \
      orca at-spi2-core speech-dispatcher \
    && rm -rf /var/lib/apt/lists/*
COPY start.sh chord-check.sh /
RUN chmod +x /start.sh /chord-check.sh
EXPOSE 5900
CMD ["/start.sh"]
