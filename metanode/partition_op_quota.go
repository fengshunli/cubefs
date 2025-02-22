// Copyright 2023 The CubeFS Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package metanode

import (
	"encoding/json"

	"github.com/cubefs/cubefs/proto"
	"github.com/cubefs/cubefs/util/log"
)

func (mp *metaPartition) batchSetInodeQuota(req *proto.BatchSetMetaserverQuotaReuqest,
	resp *proto.BatchSetMetaserverQuotaResponse) (err error) {
	for _, ino := range req.Inodes {
		var isExist bool
		var extend = NewExtend(ino)
		treeItem := mp.extendTree.Get(extend)
		inode := NewInode(ino, 0)
		retMsg := mp.getInode(inode)
		if retMsg.Status != proto.OpOk {
			log.LogErrorf("batchSetInodeQuota get inode [%v] fail.", ino)
			continue
		}
		inode = retMsg.Msg
		log.LogDebugf("batchSetInodeQuota msg [%v] inode [%v]", retMsg, inode)
		var quotaInfos = &proto.MetaQuotaInfos{
			QuotaInfoMap: make(map[uint32]*proto.MetaQuotaInfo),
		}
		var quotaInfo = &proto.MetaQuotaInfo{
			RootInode: req.IsRoot,
		}

		if treeItem == nil {
			quotaInfos.QuotaInfoMap[req.QuotaId] = quotaInfo
		} else {
			extend = treeItem.(*Extend)
			value, exist := extend.Get([]byte(proto.QuotaKey))
			if exist {
				if err = json.Unmarshal(value, &quotaInfos.QuotaInfoMap); err != nil {
					log.LogErrorf("set quota Unmarshal quotaInfos fail [%v]", err)
					resp.Status = proto.TaskFailed
					resp.Result = err.Error()
					return
				}
				oldQuotaInfo, ok := quotaInfos.QuotaInfoMap[req.QuotaId]
				if ok {
					isExist = true
					quotaInfo = oldQuotaInfo
				}
			}
			quotaInfos.QuotaInfoMap[req.QuotaId] = quotaInfo
		}
		value, err1 := json.Marshal(quotaInfos.QuotaInfoMap)
		if err1 != nil {
			log.LogErrorf("set quota marsha1 quotaInfos [%v] fail [%v]", quotaInfos, err)
			resp.Status = proto.TaskFailed
			resp.Result = err1.Error()
			err = err1
			return
		}
		extend.Put([]byte(proto.QuotaKey), value)
		if _, err = mp.putExtend(opFSMSetXAttr, extend); err != nil {
			log.LogErrorf("set quota putExtend [%v] fail [%v]", quotaInfos, err)
			resp.Status = proto.TaskFailed
			resp.Result = err.Error()
			return
		}
		if !isExist {
			mp.mqMgr.updateUsedInfo(int64(inode.Size), 1, req.QuotaId)
		}
	}
	log.LogInfof("batchSetInodeQuota quotaId [%v] mp [%v] btreeLen [%v] success", req.QuotaId, mp.config.PartitionId, mp.extendTree.Len())
	resp.Status = proto.TaskSucceeds
	return
}

func (mp *metaPartition) batchDeleteInodeQuota(req *proto.BatchDeleteMetaserverQuotaReuqest,
	resp *proto.BatchDeleteMetaserverQuotaResponse) (err error) {
	for _, ino := range req.Inodes {
		var extend = NewExtend(ino)
		var value []byte
		var deleteValue []byte
		var isRemove bool
		treeItem := mp.extendTree.Get(extend)
		inode := NewInode(ino, 0)
		retMsg := mp.getInode(inode)
		if retMsg.Status != proto.OpOk {
			log.LogErrorf("batchDeleteInodeQuota get inode [%v] fail.", ino)
			continue
		}
		inode = retMsg.Msg
		log.LogDebugf("batchDeleteInodeQuota msg [%v] inode [%v]", retMsg, inode)
		var quotaInfos = &proto.MetaQuotaInfos{
			QuotaInfoMap: make(map[uint32]*proto.MetaQuotaInfo),
		}

		if treeItem == nil {
			log.LogDebugf("batchDeleteInodeQuota inode [%v] not has extend ", ino)
			continue
		} else {
			extend = treeItem.(*Extend)
			value, exist := extend.Get([]byte(proto.QuotaKey))
			if exist {
				if err = json.Unmarshal(value, &quotaInfos.QuotaInfoMap); err != nil {
					log.LogErrorf("batchDeleteInodeQuota ino [%v] Unmarshal quotaInfos fail [%v]", ino, err)
					resp.Status = proto.TaskFailed
					resp.Result = err.Error()
					return
				}

				_, ok := quotaInfos.QuotaInfoMap[req.QuotaId]
				if ok {
					delete(quotaInfos.QuotaInfoMap, req.QuotaId)
					if len(quotaInfos.QuotaInfoMap) == 0 {
						isRemove = true
						deleteValue, err = json.Marshal(quotaInfos.QuotaInfoMap)
					}
				} else {
					log.LogDebugf("batchDeleteInodeQuota QuotaInfoMap can not find inode [%v] quota [%v]", ino, req.QuotaId)
					continue
				}
			} else {
				continue
			}
		}
		if isRemove {
			extend.Put([]byte(proto.QuotaKey), deleteValue)
			if _, err = mp.putExtend(opFSMRemoveXAttr, extend); err != nil {
				log.LogErrorf("remove quota putExtend [%v] fail [%v]", quotaInfos, err)
				resp.Status = proto.TaskFailed
				resp.Result = err.Error()
				return
			}
		} else {
			value, err = json.Marshal(quotaInfos.QuotaInfoMap)
			if err != nil {
				log.LogErrorf("batchDeleteInodeQuota marsha1 quotaInfos [%v] fail [%v]", quotaInfos, err)
				resp.Status = proto.TaskFailed
				resp.Result = err.Error()
				return
			}
			extend.Put([]byte(proto.QuotaKey), value)

			if _, err = mp.putExtend(opFSMSetXAttr, extend); err != nil {
				log.LogErrorf("set quota putExtend [%v] fail [%v]", quotaInfos, err)
				resp.Status = proto.TaskFailed
				resp.Result = err.Error()
				return
			}
		}
		mp.mqMgr.updateUsedInfo(-int64(inode.Size), -1, req.QuotaId)
	}
	log.LogInfof("batchDeleteInodeQuota quotaId [%v] success", req.QuotaId)
	resp.Status = proto.TaskSucceeds
	return
}

func (mp *metaPartition) setQuotaHbInfo(infos []*proto.QuotaHeartBeatInfo) {
	mp.mqMgr.setQuotaHbInfo(infos)
	return
}

func (mp *metaPartition) getQuotaReportInfos() (infos []*proto.QuotaReportInfo) {
	return mp.mqMgr.getQuotaReportInfos()
}

func (mp *metaPartition) statisticExtendByLoad(extend *Extend) {
	mqMgr := mp.mqMgr
	ino := NewInode(extend.GetInode(), 0)
	retMsg := mp.getInode(ino)
	if retMsg.Status != proto.OpOk {
		log.LogErrorf("statisticExtendByLoad get inode [%v] fail [%v].", extend.GetInode(), retMsg.Status)
		return
	}
	ino = retMsg.Msg
	if ino.NLink == 0 {
		return
	}
	quotaIds, isFind := mp.isExistQuota(extend.GetInode())
	if isFind {
		mqMgr.rwlock.Lock()
		defer mqMgr.rwlock.Unlock()
		for _, quotaId := range quotaIds {
			var baseInfo proto.QuotaUsedInfo
			value, isFind := mqMgr.statisticBase.Load(quotaId)
			if isFind {
				baseInfo = value.(proto.QuotaUsedInfo)
			}
			baseInfo.UsedBytes += int64(ino.Size)
			baseInfo.UsedFiles += 1
			mqMgr.statisticBase.Store(quotaId, baseInfo)
			log.LogDebugf("[statisticExtendByLoad] quotaId [%v] baseInfo [%v]", quotaId, baseInfo)

		}
	}
	log.LogInfof("statisticExtendByLoad ino [%v] isFind [%v].", ino.Inode, isFind)
	return
}

func (mp *metaPartition) statisticExtendByStore(extend *Extend, inodeTree *BTree) {
	mqMgr := mp.mqMgr
	ino := NewInode(extend.GetInode(), 0)
	item := inodeTree.Get(ino)
	if item == nil {
		_, isleader := mp.IsLeader()
		log.LogErrorf("statisticExtendByStore mp [%v] leader [%v] inode [%v] is not exist.",
			mp.config.PartitionId, isleader, extend.GetInode())
		return
	}
	ino = item.(*Inode)
	if ino.NLink == 0 {
		return
	}
	value, exist := extend.Get([]byte(proto.QuotaKey))
	if !exist {
		log.LogDebugf("statisticExtendByStore get quota key failed, mp [%v] inode [%v]", mp.config.PartitionId, extend.GetInode())
		return
	}
	var quotaInfos = &proto.MetaQuotaInfos{
		QuotaInfoMap: make(map[uint32]*proto.MetaQuotaInfo),
	}
	if err := json.Unmarshal(value, &quotaInfos.QuotaInfoMap); err != nil {
		log.LogErrorf("statisticExtendByStore inode [%v] Unmarshal quotaInfos fail [%v]", extend.GetInode(), err)
		return
	}
	mqMgr.rwlock.Lock()
	defer mqMgr.rwlock.Unlock()
	for quotaId := range quotaInfos.QuotaInfoMap {
		var baseInfo proto.QuotaUsedInfo
		value, isFind := mqMgr.statisticRebuildBase.Load(quotaId)
		if isFind {
			baseInfo = value.(proto.QuotaUsedInfo)
		}
		baseInfo.UsedBytes += int64(ino.Size)
		baseInfo.UsedFiles += 1
		mqMgr.statisticRebuildBase.Store(quotaId, baseInfo)
		log.LogDebugf("[statisticExtendByStore] mp [%v] quotaId [%v] inode [%v] baseInfo [%v]",
			mp.config.PartitionId, quotaId, extend.GetInode(), baseInfo)
	}
	log.LogDebugf("statisticExtendByStore mp [%v] inode [%v] success.", mp.config.PartitionId, extend.GetInode())
	return
}

func (mp *metaPartition) updateUsedInfo(size int64, files int64, ino uint64) {
	quotaIds, isFind := mp.isExistQuota(ino)
	if isFind {
		log.LogInfof("updateUsedInfo ino [%v] quotaIds [%v] size [%v] files [%v]", ino, quotaIds, size, files)
		for _, quotaId := range quotaIds {
			mp.mqMgr.updateUsedInfo(size, files, quotaId)
		}
	}
	return
}

func (mp *metaPartition) isExistQuota(ino uint64) (quotaIds []uint32, isFind bool) {
	var extend = NewExtend(ino)
	treeItem := mp.extendTree.Get(extend)
	if treeItem == nil {
		isFind = false
		return
	}
	extend = treeItem.(*Extend)
	value, exist := extend.Get([]byte(proto.QuotaKey))
	if !exist {
		isFind = false
		return
	}
	var quotaInfos = &proto.MetaQuotaInfos{
		QuotaInfoMap: make(map[uint32]*proto.MetaQuotaInfo),
	}
	if err := json.Unmarshal(value, &quotaInfos.QuotaInfoMap); err != nil {
		log.LogErrorf("set quota inode [%v] Unmarshal quotaInfos fail [%v]", ino, err)
		isFind = false
		return
	}
	isFind = true
	quotaInfos.RLock()
	for quotaId := range quotaInfos.QuotaInfoMap {
		quotaIds = append(quotaIds, quotaId)
	}
	quotaInfos.RUnlock()
	log.LogInfof("isExistQuota inode:[%v] quotaIds [%v] isFind[%v]", ino, quotaIds, isFind)
	return
}

func (mp *metaPartition) isOverQuota(ino uint64, size bool, files bool) (status uint8) {
	quotaIds, isFind := mp.isExistQuota(ino)
	if isFind {
		for _, quotaId := range quotaIds {
			status = mp.mqMgr.IsOverQuota(size, files, quotaId)
			if status != 0 {
				log.LogWarnf("isOverQuota ino [%v] quotaId [%v] size [%v] files[%v] status[%v]", ino, quotaId, size, files, status)
				return
			}
		}
	}
	return
}

func (mp *metaPartition) getInodeQuota(inode uint64, p *Packet) (err error) {
	var extend = NewExtend(inode)
	var quotaInfos = &proto.MetaQuotaInfos{
		QuotaInfoMap: make(map[uint32]*proto.MetaQuotaInfo),
	}
	var (
		value []byte
		exist bool
	)
	treeItem := mp.extendTree.CopyGet(extend)
	if treeItem == nil {
		goto handleRsp
	}
	extend = treeItem.(*Extend)

	value, exist = extend.Get([]byte(proto.QuotaKey))
	if exist {
		if err = json.Unmarshal(value, &quotaInfos.QuotaInfoMap); err != nil {
			log.LogErrorf("getInodeQuota inode [%v] Unmarshal quotaInfos fail [%v]", inode, err)
			p.PacketErrorWithBody(proto.OpErr, []byte(err.Error()))
			return
		}
	}
handleRsp:
	var response = &proto.GetInodeQuotaResponse{}
	log.LogInfof("getInodeQuota indoe %v ,map %v", inode, quotaInfos.QuotaInfoMap)
	response.MetaQuotaInfoMap = quotaInfos.QuotaInfoMap

	encoded, err := json.Marshal(response)
	if err != nil {
		p.PacketErrorWithBody(proto.OpErr, []byte(err.Error()))
		return
	}
	p.PacketOkWithBody(encoded)
	return
}

func (mp *metaPartition) getInodeQuotaIds(inode uint64) (quotaIds []uint32, err error) {
	log.LogInfof("getInodeQuotaIds mp [%v] treeLen[%v]", mp.config.PartitionId, mp.extendTree.Len())
	treeItem := mp.extendTree.CopyGet(NewExtend(inode))
	if treeItem == nil {
		return
	}
	extend := treeItem.(*Extend)
	var quotaInfos = &proto.MetaQuotaInfos{
		QuotaInfoMap: make(map[uint32]*proto.MetaQuotaInfo),
	}
	value, exist := extend.Get([]byte(proto.QuotaKey))
	if exist {
		if err = json.Unmarshal(value, &quotaInfos.QuotaInfoMap); err != nil {
			log.LogErrorf("getInodeQuota inode [%v] Unmarshal quotaInfos fail [%v]", inode, err)
			return
		}
		for k := range quotaInfos.QuotaInfoMap {
			quotaIds = append(quotaIds, k)
		}
	}
	log.LogInfof("getInodeQuotaIds inode [%v] quotaIds [%v] exist [%v]", inode, quotaIds, exist)
	return
}

func (mp *metaPartition) setInodeQuota(quotaIds []uint32, inode uint64) {
	var extend = NewExtend(inode)
	var quotaInfos = &proto.MetaQuotaInfos{
		QuotaInfoMap: make(map[uint32]*proto.MetaQuotaInfo),
	}
	for _, quotaId := range quotaIds {
		var quotaInfo = &proto.MetaQuotaInfo{
			RootInode: false,
		}
		quotaInfos.QuotaInfoMap[quotaId] = quotaInfo
	}
	value, err := json.Marshal(quotaInfos.QuotaInfoMap)
	if err != nil {
		log.LogErrorf("setInodeQuota marsha1 quotaInfos [%v] fail [%v]", quotaInfos, err)
		return
	}
	extend.Put([]byte(proto.QuotaKey), value)
	treeItem := mp.extendTree.CopyGet(extend)
	var e *Extend
	if treeItem == nil {
		e = NewExtend(extend.inode)
		mp.extendTree.ReplaceOrInsert(e, true)
	} else {
		e = treeItem.(*Extend)
	}
	e.Merge(extend, true)
	log.LogInfof("setInodeQuota Inode [%v] quota [%v] success.", inode, quotaIds)
	return
}
