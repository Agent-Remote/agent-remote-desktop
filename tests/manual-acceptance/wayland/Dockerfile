# Disposable Wayland desktop for agent-remote-desktop acceptance: headless
# sway (wlroots), the app on GTK's Wayland backend, wtype for the virtual
# keyboard protocol and wayvnc to look at it. Nothing persists.
FROM debian:bookworm-slim
ENV DEBIAN_FRONTEND=noninteractive LANG=C.UTF-8
RUN apt-get update -qq && apt-get install -y -qq --no-install-recommends \
      sway wayvnc wtype grim libgtk-3-0 libayatana-appindicator3-1 libpango-1.0-0 \
      dbus-x11 ca-certificates fonts-noto-cjk fonts-dejavu-core \
    && rm -rf /var/lib/apt/lists/*
COPY start.sh chord-check.sh /
RUN chmod +x /start.sh /chord-check.sh && useradd -m tester
# sway refuses to run as root.
USER tester
EXPOSE 5900
CMD ["/start.sh"]
