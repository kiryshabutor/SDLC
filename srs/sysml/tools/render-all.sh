#!/bin/sh
# Пересобирает все PNG из srs/sysml/*.sysml в srs/images/.
# Запуск из каталога srs/sysml:  sh tools/render-all.sh
set -e
cd "$(dirname "$0")/.."
docker build -q -t sysml-render:0.62 tools >/dev/null
mkdir -p ../images
docker run --rm -u "$(id -u):$(id -g)" -e HOME=/tmp -v "$PWD/..:/srs" -w /srs/sysml sysml-render:0.62 \
  python tools/render.py /srs/images \
    ui-kit.sysml context.sysml use-cases.sysml \
    ui-01-auth.sysml ui-02-wishlist.sysml ui-03-search.sysml \
    ui-04-game.sysml ui-05-notifications.sysml ui-06-profile.sysml -- \
    'context|Context::ctx|--view=interconnection|--style=LR' \
    'use-cases|UseCases|--view=case|--style=LR' \
    'ui-01-auth|UI_01_Auth::auth|--view=interconnection|wire' \
    'ui-02-wishlist|UI_02_Wishlist::wishlist|--view=interconnection|wire' \
    'ui-03-search|UI_03_Search::search|--view=interconnection|wire' \
    'ui-04-game|UI_04_Game::game|--view=interconnection|wire' \
    'ui-05-notifications|UI_05_Notifications::notifications|--view=interconnection|wire' \
    'ui-06-profile|UI_06_Profile::profile|--view=interconnection|wire'
