# Copyright 2022 DigitalOcean
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM alpine:3.15

# e2fsprogs-extra is required for resize2fs used for the resize operation
# blkid: block device identification tool from util-linux
RUN apk add --no-cache ca-certificates \
                       cryptsetup \
                       e2fsprogs \
                       findmnt \
                       xfsprogs \
                       xfsprogs-extra \
                       blkid \
                       e2fsprogs-extra

ADD do-csi-plugin /bin/

ENTRYPOINT ["/bin/do-csi-plugin"]
