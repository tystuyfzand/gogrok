PACKAGE_ARCH="$ARCH"

if [ "$PACKAGE_ARCH" = "arm" ]; then
  PACKAGE_ARCH="armhf"
elif [ "$PACKAGE_ARCH" = "386" ]; then
  PACKAGE_ARCH="i386"
fi

fpm -s dir -t deb -p /build/gogrok_${DRONE_TAG}_${ARCH}.deb \
    -n gogrok -v "$DRONE_SEMVER" -a "$PACKAGE_ARCH" \
    --deb-priority optional --force \
    --deb-compression gz --verbose \
    --description "Gogrok: Open-source ngrok alternative" \
    -m "Tyler Stuyfzand <admin@meow.tf>" --vendor "Meow.tf" \
    /build/gogrok_linux_${ARCH}=/usr/bin/gogrok

fpm -s dir -t rpm -p /build/gogrok_${DRONE_TAG}_${ARCH}.rpm \
    -n gogrok -v $DRONE_SEMVER -a "$PACKAGE_ARCH" \
    --description "Gogrok: Open-source ngrok alternative" \
    -m "Tyler Stuyfzand <admin@meow.tf>" --vendor "Meow.tf" \
    /build/gogrok_linux_${ARCH}=/usr/bin/gogrok