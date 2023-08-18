# encoding=utf8

import sys
import os
import os.path
import json
import time
import datetime
import subprocess
import zipfile
from os.path import isfile
from argparse import ArgumentParser

class DockerTool(object):
    def __init__(self, rootDir):
        self.rootDir = "."
        self.curdir = os.path.dirname(os.path.abspath(sys.argv[0]))
        self.tempDir = ".tmp"
        if rootDir:
            self.rootDir = rootDir

        tempDir = os.path.join(self.rootDir, self.tempDir)
        os.system("mkdir -p %s" % tempDir)

    def getMetadata(self, imageID, needPull=False):    
        tempId = imageID.replace(":","_").replace("/","_")
        filePath = "{}/{}/{}.tar".format(self.rootDir, self.tempDir, tempId)

        if needPull:
            print("pull image: " + imageID)
            ret = os.system("cd {} && docker pull {} && docker save -o {} {} ".format(self.rootDir, imageID, filePath, imageID))
        else:
            ret = os.system("cd {} && docker save -o {} {} ".format(self.rootDir, filePath, imageID))

        imageLaysDir = os.path.join(self.rootDir, self.tempDir, tempId)

        if not os.path.exists(imageLaysDir):
            os.system("mkdir -p %s" % imageLaysDir)
        ret = os.system("tar -xf %s  -C %s && ls -l |grep ^d  > /dev/null && rm -rf %s " % (filePath, imageLaysDir, filePath) )
        manifestFile = os.path.join(imageLaysDir, "manifest.json")
        maniData = {}
        with open(manifestFile, "r") as fd:
            rawdata = fd.read()
            maniData = json.loads(rawdata)

        # 默认获取第一个镜像
        maniData = maniData[0]
        imageTag = tempId[:(str.rfind(tempId, "_") + 1)]
        imageVer = imageID.split(':')[1]
        return maniData, imageLaysDir, imageTag, imageVer

    def imageDiff(self, latestImageId, originImageId, pullImage):

        print("old image: %s" % originImageId)
        originMeta, originDir,originTag, originVer = self.getMetadata(originImageId, pullImage)

        print("new image: %s" % latestImageId)
        latestMeta, imageLaysDir, imageTagId, imageVer = self.getMetadata(latestImageId, pullImage)

        difflayers = []
        existLayers = []
        existLayerFile = "{}/existlayers".format(imageLaysDir)

        for x in latestMeta.get("Layers"):
            layerId = x.split("/")[0]
            if x not in originMeta.get("Layers"):
                difflayers.append(layerId)
            else:
                existLayers.append(layerId)

        with open(existLayerFile, "w") as fd:
            for layer in existLayers:
                fd.write(layer)

        allfiles = [latestMeta.get("Config")]
        allfiles.extend(["manifest.json", "repositories","existlayers"])
        allfiles.extend(difflayers)

        diffTarFilename = "{}_{}__{}__diff.tar.gz".format(imageTagId,originVer,imageVer)

        cmdstr = "cd {} && tar -zcf {} ".format(imageLaysDir, diffTarFilename)
        for filename in allfiles:
            cmdstr += " {} ".format(filename)
        os.system(cmdstr)

        absRootPath = os.path.abspath(self.rootDir)

        cmdstr3 = "cd {} && mv {} {}".format(imageLaysDir, diffTarFilename, absRootPath)
        os.system(cmdstr3)

        print("create success: " + absRootPath + "/" + diffTarFilename)

        cmdStr4 = "cd {} && rm -rf {}".format(absRootPath, self.tempDir)
        os.system(cmdStr4)


def getArguments():
    parser = ArgumentParser(description = "tool")
    parser.add_argument("-n","--newId", dest = "newId", default = ".", help = "The imageID")
    parser.add_argument("-o","--oldId", dest = "oldId", default = ".", help = "The old imageId")
    parser.add_argument("-d","--rootDir", dest = "rootDir", default = ".", help = "The root dir")
    parser.add_argument("-p","--pullImage", dest ="pullImage", default= False, help = "pull image first")
    return parser.parse_args()

if __name__ == '__main__':

    time0 = time.time()
    args = getArguments()
    tool = DockerTool(args.rootDir)

    print("work dir is %s" % (tool.rootDir))

    tool.imageDiff(args.newId, args.oldId, args.pullImage)
    print("cost %s seconds !" % (time.time() - time0))
