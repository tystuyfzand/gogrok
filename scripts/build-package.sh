fpm -s dir -t deb -p /build/gogrok_${DRONE_TAG}_${ARCH}.deb \
    -n gogrok -v $DRONE_SEMVER -a $ARCH \
    --deb-priority optional --force \
    --deb-compression gz --verbose \
    --description "Gogrok: Open-source ngrok alternative" \
    -m "Tyler Stuyfzand <admin@meow.tf>" --vendor "Paste.ee" \
    -a $ARCH /build/gogrok_linux_${ARCH}=/usr/bin/gogrok

fpm -s dir -t rpm -p /build/gogrok_${DRONE_TAG}_${ARCH}.rpm \
    -n gogrok -v $DRONE_SEMVER -a $ARCH \
    --description "Gogrok: Open-source ngrok alternative" \
    -m "Tyler Stuyfzand <admin@meow.tf>" --vendor "Paste.ee" \
    -a $ARCH /build/gogrok_linux_${ARCH}=/usr/bin/gogrok