# np2bio
Spotify の Now Playing を Twitter の bio に設定するボット

## Setup
* `.env.sample` を参考にして、必要な値を `.env` に設定してください。
* Spotify Web API について、コールバック先 URL を `http://localhost:3000` または `http://稼働するコンピュータの IP アドレス:3000` に設定してください。これは、Spotify の `refresh_token` を取得する際に必要で、設定後は使用されません。

## License
MIT