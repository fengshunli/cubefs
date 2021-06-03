#!/usr/bin/python
import os
# for python < 2.7
os.system("wget --quiet -O argparse.py http://storage.jd.local/cfs-client-lib/argparse.py")
import argparse

parser = argparse.ArgumentParser(prog = 'install', description = 
'''
This script will install kernel bypass client for ChubaoFS(CFS).
CFS is virtually mounted at mountPoint. LD_PRELOAD is utilized to 
hook libc wrapper functions of file system calls. All functons with 
paths or file descriptors belong to the mount point are distributed 
to CFS.

Usage:
Starting a process with environment variable LD_PRELOAD=libcfsclient.so 
CFS_CONFIG_PATH={configDir}/cfs_client.ini will enable CFS support.
(e.g. Surpose mountPoint=/cfs_test, LD_PRELOAD= CFS_CONFIG_PATH= 
cp -r someLocalDir /cfs_test)

Notice:
Some shell features are not supported, e.g. cd, file globbing.
And mv is not supported for coreutils under version 8.31.
''')
parser.add_argument('--configDir', default='/export/servers/cfs', help='directory for client config file (default: /export/servers/cfs)')
parser.add_argument('--mountPoint', default='/export/data/mysql', help='mount point (default: /export/data/mysql)')
parser.add_argument('--ignorePath', help='add ignore path at mountPoint, separated by commas')
parser.add_argument('--masterAddr', required=True)
parser.add_argument('--volName', required=True)
parser.add_argument('--owner', required=True)
parser.add_argument('--followerRead', default='false', help='enable to read from follower nodes (default: false)')
parser.add_argument('--logDir', default='/export/data/mysql/log', help='log dir (default: /export/data/mysql/log)')
parser.add_argument('--logLevel', default='info', help='log level, debug|info|warn|error (default: info)')
parser.add_argument('--profPort', default='10094', help='port for profiling (default: 10094)')
#parser.add_argument('--tracingSamplerType', default='probabilistic', help='(default: probabilistic)')
#parser.add_argument('--tracingSamplerParam', default='0.1', help='(default: 0.1)')
parser.add_argument('--tracingSamplerType', default='const', help='(default: const)')
parser.add_argument('--tracingSamplerParam', default='1', help='(default: 1)')
parser.add_argument('--tracingReportAddr')

parser.add_argument('--libDir', default='/usr/lib64', help='(default: /usr/lib64)')
parser.add_argument('--appName', default='mysql', help='(default: mysql)')
args = parser.parse_args()

os.system('mkdir -p {0}'.format(args.configDir))
with open('{0}/cfs_client.ini'.format(args.configDir), 'w') as f:
    f.write('mountPoint={0}\nignorePath={1}\nmasterAddr={2}\nvolName={3}\nowner={4}\nfollowerRead={5}\nlogDir={6}\nlogLevel={7}\nprofPort={8}\ntracingSamplerType={9}\ntracingSamplerParam={10}\ntracingReportAddr={11}\nappName={12}'.format(args.mountPoint, args.ignorePath, args.masterAddr, args.volName, args.owner, args.followerRead, args.logDir, args.logLevel, args.profPort, args.tracingSamplerType, args.tracingSamplerParam, args.tracingReportAddr, args.appName))

os.system('wget --quiet -O libcfs.tar.gz http://storage.jd.local/cfs-client-lib/libcfs.tar.gz && tar xzf libcfs.tar.gz && chmod 755 libcfssdk.so libcfsclient.so && mv libcfssdk.so libcfsclient.so {0} && rm -f libcfs.tar.gz argparse.py*'.format(args.libDir))