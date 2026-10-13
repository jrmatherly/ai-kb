class AiKbLite < Formula
  desc "Knowledge base management system, single-binary Lite edition"
  homepage "https://github.com/jrmatherly/ai-kb"
  version "0.3.6-test"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/jrmatherly/ai-kb/releases/download/v#{version}/ai-kb-lite_v#{version}_darwin_arm64.tar.gz"
      sha256 "1da2d4eef99e5cf8aa7a58501baa059e9e20482e1bd65a36a82321a89926c104"
    end
    on_intel do
      url "https://github.com/jrmatherly/ai-kb/releases/download/v#{version}/ai-kb-lite_v#{version}_darwin_amd64.tar.gz"
      sha256 "c187e16ac7671a615f012c82ebd89786e11fcf67cccc773eff175e4bdf7c9c06"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/jrmatherly/ai-kb/releases/download/v#{version}/ai-kb-lite_v#{version}_linux_arm64.tar.gz"
      sha256 "bc4e184da005b60d1e8c037a61c58e643ebdc9bf14470fae6cd6227f52f02f1c"
    end
    on_intel do
      url "https://github.com/jrmatherly/ai-kb/releases/download/v#{version}/ai-kb-lite_v#{version}_linux_amd64.tar.gz"
      sha256 "cb34c50fb5b05555fca16084ffc7710524ff78badb3b1b82474eb89d21545d6e"
    end
  end

  def install
    libexec.install "ai-kb-lite"
    pkgshare.install "web" if File.directory?("web")
    pkgshare.install "config" if File.directory?("config")
    pkgshare.install ".env.lite.example"
    doc.install "README.md"
    pkgshare.install "migrations" if File.directory?("migrations")

    (bin/"ai-kb-lite").write <<~SH
      #!/bin/bash
      CONFIG_DIR="${AIKB_CONFIG_DIR:-${XDG_CONFIG_HOME:-$HOME/.config}/aikb}"
      DATA_DIR="${AIKB_DATA_DIR:-${XDG_DATA_HOME:-$HOME/.local/share}/aikb}"

      mkdir -p "$DATA_DIR/files" "$CONFIG_DIR/config" 2>/dev/null

      if [ ! -f "$CONFIG_DIR/config/config.yaml" ]; then
        cp -r "#{pkgshare}/config/" "$CONFIG_DIR/config/"
      fi

      if [ ! -d "$CONFIG_DIR/migrations" ] && [ -d "#{pkgshare}/migrations" ]; then
        ln -sf "#{pkgshare}/migrations" "$CONFIG_DIR/migrations"
      fi

      if [ ! -f "$CONFIG_DIR/.env.lite" ]; then
        cp "#{pkgshare}/.env.lite.example" "$CONFIG_DIR/.env.lite"
        sed -i.bak -e "s|DB_PATH=.*|DB_PATH=$DATA_DIR/aikb.db|" -e "s|LOCAL_STORAGE_BASE_DIR=.*|LOCAL_STORAGE_BASE_DIR=$DATA_DIR/files|" "$CONFIG_DIR/.env.lite"
        rm -f "$CONFIG_DIR/.env.lite.bak"
        echo ""
        echo "Created config file: $CONFIG_DIR/.env.lite"
        echo "Edit it as needed (for example the LLM endpoint or the secret keys)."
        echo ""
      fi

      set -a
      source "$CONFIG_DIR/.env.lite"
      set +a

      export DB_PATH="${DB_PATH:-$DATA_DIR/aikb.db}"
      export LOCAL_STORAGE_BASE_DIR="${LOCAL_STORAGE_BASE_DIR:-$DATA_DIR/files}"
      export AIKB_WEB_DIR="${AIKB_WEB_DIR:-#{pkgshare}/web}"

      cd "$CONFIG_DIR"
      exec "#{libexec}/ai-kb-lite" "$@"
    SH
  end

  def post_install
    (var/"aikb").mkpath
    (var/"log").mkpath
  end

  service do
    run [bin/"ai-kb-lite"]
    keep_alive true
    working_dir var/"aikb"
    log_path var/"log/ai-kb-lite.log"
    error_log_path var/"log/ai-kb-lite.log"
  end

  def caveats
    <<~EOS
      Run in the foreground:
        ai-kb-lite

      Background service (recommended):
        brew services start ai-kb-lite   # start now and at login
        brew services stop ai-kb-lite    # stop
        brew services restart ai-kb-lite # restart
        brew services info ai-kb-lite    # show status

      Logs:
        #{var}/log/ai-kb-lite.log

      The first run creates the config file:
        ~/.config/aikb/.env.lite

      Data is stored in:
        ~/.local/share/aikb/

      To change the config (LLM endpoint, secret keys and so on):
        $EDITOR ~/.config/aikb/.env.lite
        brew services restart ai-kb-lite
    EOS
  end

  test do
    assert_predicate bin/"ai-kb-lite", :executable?
  end
end
