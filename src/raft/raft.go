package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import "sync"
import (
	"labrpc"
	"time"
	"math/rand"
	"sync/atomic"
	"bytes"
	"labgob"
	"fmt"
)

// import "bytes"
// import "labgob"

const (
	ServerStateNone = iota
	ServerStateLeader
	ServerStateCandidate
	ServerStateFollower
)
var StateName = []string{"None", "Leader", "Candidate", "Follower"}

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
//
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
}

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu          		sync.Mutex          // Lock to protect shared access to this peer's state, goroutine cannot block after acquire lock.
	// muCond      		*sync.Cond
	// followerCommitMu 	sync.Mutex
	peers       		[]*labrpc.ClientEnd // RPC end points of all peers
	persister   		*Persister          // Object to hold this peer's persisted state
	me          		int                 // this peer's index into peers[]
	serverCount 		int
	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	// Persistent state for all servers.
	currentTerm 	int
	voteFor     	int
	log         	[]interface{}
	logTerm     	[]int


	// Volatile state on all servers.
	commitIndex 	int
	lastApplied 	int

	// Volatile state on leaders
	nextIndex   	[]int
	matchIndex  	[]int

	// Server state.
	state       	int


	// Read only field
	electionTimePeriod  time.Duration
	electionTimeWave  	time.Duration
	heartBeatPeriod  	time.Duration

	// Timer
	timerMgr     		*TimerMgr

	applyCh				chan ApplyMsg
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	var term int
	var isLeader bool
	// Your code here (2A).
	if rf.state == ServerStateLeader {
		isLeader = true
	} else {
		isLeader = false
	}

	term = rf.currentTerm

	return term, isLeader
}


//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	//rf.mu.Lock()
	//defer rf.mu.Unlock()

	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)

	e.Encode(rf.currentTerm)
	e.Encode(rf.voteFor)
	e.Encode(rf.log)
	e.Encode(rf.logTerm)
	data := w.Bytes()
	rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	currentTerm := 0
	voteFor := 0
	var log []interface{}
	var logTerm []int

	if d.Decode(&currentTerm) != nil ||
		d.Decode(&voteFor) != nil ||
		d.Decode(&log) != nil ||
		d.Decode(&logTerm) != nil {
	   	DPrintf(LogLevelError, rf, "Read persist error.")
	} else {
		rf.currentTerm = currentTerm
		rf.voteFor = voteFor
		rf.log = log
		rf.logTerm = logTerm
		rf.lastApplied = len(log) - 1
	}
}




//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term            int
	CandidateIndex  int
	LastLogTerm     int
	LastLogIndex    int
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term            int
	VoteGranted     bool
}


type AppendEntriesArgs struct {
	Term            int
	LeaderId        int
	PrevLogIndex    int
	PrevLogTerm     int
	Entries         []interface{}
	LeaderCommit    int
}

type AppendEntriesReply struct {
	Term            int
	Success         bool
	CommitIndex		int
}
func (rf *Raft) LeaderRequestVoteHandler() {}

func (rf *Raft) CandidateRequestVoteHandler() {}

func (rf *Raft) FollowerRequestVoteHandler() {}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if args.Term < rf.currentTerm {
		reply.VoteGranted = false
		reply.Term = rf.currentTerm
		DPrintf(LogLevelDebug, rf, "rej, %v %v %v\n", args.Term, rf.currentTerm, rf.voteFor)
	} else if args.LastLogTerm > rf.logTerm[rf.lastApplied] || // at least up-to-date
		(args.LastLogTerm == rf.logTerm[rf.lastApplied] && args.LastLogIndex > rf.lastApplied) ||
		(args.LastLogTerm == rf.logTerm[rf.lastApplied] && args.LastLogIndex == rf.lastApplied && rf.state == ServerStateCandidate) ||
		(args.LastLogTerm == rf.logTerm[rf.lastApplied] && args.LastLogIndex == rf.lastApplied && rf.state == ServerStateFollower && rf.voteFor == -1) ||
		(args.LastLogTerm == rf.logTerm[rf.lastApplied] && args.LastLogIndex == rf.lastApplied && rf.state == ServerStateFollower && rf.voteFor == args.CandidateIndex)	{ // at least up-to-date
		/* (args.Term == rf.currentTerm && rf.state == ServerStateCandidate && rf.voteFor == args.CandidateIndex) */
		reply.VoteGranted = true
		rf.currentTerm = args.Term
		reply.Term = rf.currentTerm

		rf.ChangeState(rf.state, ServerStateFollower)
		rf.voteFor = args.CandidateIndex
		rf.persist()
	} else {
		reply.VoteGranted = false
		reply.Term = rf.currentTerm
		DPrintf(LogLevelDebug, rf, "rej 2")
	}
	DPrintf(LogLevelInfo, rf, "Get RequestVote rpc from %v, Term %v, Reply %v", args.CandidateIndex, args.Term, *reply)


}
func (rf *Raft) AppendEntriesPreCheck(args *AppendEntriesArgs) bool {
	return args.PrevLogIndex <= rf.lastApplied &&
		args.PrevLogTerm == rf.logTerm[args.PrevLogIndex]

}
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()


	reply.CommitIndex = rf.commitIndex

	switch rf.state {
	case ServerStateLeader:
		if args.Term < rf.currentTerm {
			reply.Term = rf.currentTerm
			reply.Success = false
			DPrintf(LogLevelInfo, rf, "Get AppendEntries RPC from %v, Term %v, Reply %v\n", args.LeaderId, args.Term, *reply)
			return
		}

		if args.Term == rf.currentTerm {
			DPrintf(LogLevelWarning, rf, "2 leader with identical term")
		}
		rf.ChangeState(rf.state, ServerStateFollower)
		rf.voteFor = args.LeaderId
		rf.currentTerm = args.Term

		reply.Term = args.Term
		reply.Success = true
	case ServerStateCandidate:
		if args.Term >= rf.currentTerm - 1 {
			rf.ChangeState(rf.state, ServerStateFollower)
			rf.voteFor = args.LeaderId
			rf.currentTerm = args.Term

			reply.Term = args.Term
			reply.Success = true
		} else {
			reply.Term = rf.currentTerm
			reply.Success = false
		}

	case ServerStateFollower:
		if args.Term >= rf.currentTerm {
			/* (args.Term == rf.currentTerm && rf.voteFor == args.LeaderId) {*/
			rf.ResetCurrentTimer()
			rf.voteFor = args.LeaderId
			rf.currentTerm = args.Term

			reply.Term = args.Term
			reply.Success = true
		} else {
			reply.Term = rf.currentTerm
			reply.Success = false
		}
	}

	if !rf.AppendEntriesPreCheck(args) {
		reply.Success = false
		reply.Term = rf.currentTerm
		DPrintf(LogLevelInfo, rf, "Get AppendEntries RPC from %v, Term %v, Reply %v, Pre check fail.\n", args.LeaderId, args.Term, *reply)
		return
	}

	if !reply.Success {
		DPrintf(LogLevelInfo, rf, "Get AppendEntries RPC from %v, Term %v, Reply %v\n", args.LeaderId, args.Term, *reply)
		return
	}

	// append log
	if len(rf.log) != len(rf.logTerm) {
		DPrintf(LogLevelError, rf, "length of Raft.log mismatch with Raft.logTerm")
		return
	}

	for idx := args.PrevLogIndex + 1; idx <= args.PrevLogIndex + len(args.Entries); idx++ {
		if idx < len(rf.log) {
			rf.log[idx] = args.Entries[idx - args.PrevLogIndex - 1]
			rf.logTerm[idx] = rf.currentTerm
		} else {
			rf.log = append(rf.log, args.Entries[idx - args.PrevLogIndex - 1])
			rf.logTerm = append(rf.logTerm, rf.currentTerm)
		}
	}
	rf.lastApplied = args.PrevLogIndex + len(args.Entries)
	rf.log = rf.log[:rf.lastApplied + 1]
	rf.logTerm = rf.logTerm[:rf.lastApplied + 1]

	if rf.commitIndex > rf.lastApplied {
		rf.commitIndex = rf.lastApplied
		reply.CommitIndex = rf.commitIndex
	}

	// change commit status
	if args.LeaderCommit < rf.commitIndex {
		DPrintf(LogLevelWarning, rf, "commit log index ahead of leader, possibly inconsistent")
		rf.commitIndex = args.LeaderCommit
		reply.CommitIndex = rf.commitIndex
	}

	if args.LeaderCommit > rf.commitIndex {
		// rf.followerCommitMu.Lock()
		go func(lastCommit int, newCommit int) {
			// defer rf.followerCommitMu.Unlock()
			for idx := lastCommit + 1; idx <= newCommit; idx++ {
				rf.mu.Lock()
				msg := ApplyMsg{true, rf.log[idx], idx}
				rf.mu.Unlock()
				select {
				case rf.applyCh <- msg:

				case <-time.After(time.Second):
					fmt.Println("Write applyCh timeout!!")
				}
			}
		}(rf.commitIndex, Min(args.LeaderCommit, rf.lastApplied))

		rf.commitIndex = Min(args.LeaderCommit, rf.lastApplied)
		reply.CommitIndex = rf.commitIndex
	}

	rf.persist()
	DPrintf(LogLevelInfo, rf, "Get AppendEntries RPC from %v, Term %v, Reply %v, applied|commit %v|%v\n", args.LeaderId, args.Term, *reply, rf.lastApplied, rf.commitIndex)

}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	rf.mu.Lock()
	DPrintf(LogLevelInfo, rf, "Send request vote to server %v\n", server)
	rf.mu.Unlock()
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}



func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	rf.mu.Lock()
	DPrintf(LogLevelInfo, rf, "Send Append Entries to server|prevIndex|prevTerm %v|%v|%v\n", server, args.PrevLogIndex, args.PrevLogTerm)
	rf.mu.Unlock()
	if args == nil {
		panic("args nil error")
	}
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

func (rf *Raft) sendAppendEntriesReliable(server int, args *AppendEntriesArgs, reply *AppendEntriesReply,  elapseSignature int) {
	for {
		ret := rf.sendAppendEntries(server, args, reply)
		if ret {
			break
		} else if elapseSignature != rf.timerMgr.GetTimerId() { // is current loop is viable.
			break
		}
	}
}

func(rf * Raft) sendAppendEntriesCallback(appendEntriesReplyList []AppendEntriesReply) {
	// TODO

	panic("not implemented")
}
//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).
	if rf.state != ServerStateLeader {
		isLeader = false
		return index, term, isLeader
	}

	rf.log = append(rf.log, command)
	rf.logTerm = append(rf.logTerm, rf.currentTerm)
	rf.lastApplied++
	index = rf.lastApplied
	term = rf.currentTerm

	rf.persist()
	DPrintf(LogLevelInfo, rf, "start consensus, index|command %v|%v\n", index, command)

	return index, term, isLeader
}

//
// the tester calls Kill() when a Raft instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (rf *Raft) Kill() {
	// Your code here, if desired.
	//DPrintf(LogLevelInfo, rf, "Kill")
	// rf.timerMgr.stop = true
	rf.mu.Lock()
	defer rf.mu.Unlock()
	DPrintf(LogLevelInfo, rf, "Server Stop.")

	rf.persist()
	rf.timerMgr.stop = true
}

// Generate Duration range in range
// [rf.electionTimeOut - rf.electionTimeWave, rf.electionTimeOut + rf.electionTimeWave]
func (rf *Raft) GetCandidateLoopPeriod() time.Duration {
	return (rf.electionTimePeriod - rf.electionTimeWave) +
		time.Duration(int64(time.Nanosecond) * rand.Int63n(rf.electionTimeWave.Nanoseconds() * 2))
}

func (rf *Raft) GetLeaderLoopPeriod() time.Duration {
	return rf.heartBeatPeriod
}

func (rf *Raft) GetFollowerLoopPeriod() time.Duration {
	return rf.GetCandidateLoopPeriod()
}

func (rf *Raft) StartElection(elapseSignature int) {
	// TODO expire termination.
	// TODO Lock service.

	rf.mu.Lock()
	if rf.state != ServerStateCandidate {
		DPrintf(LogLevelWarning, rf,"Raft state is not leader.")
		rf.mu.Unlock()
		return
	}
	// rf.currentTerm++
	rf.voteFor = rf.me

	DPrintf(LogLevelInfo, rf, "Election start, current term %v\n", rf.currentTerm)

	requestVoteArgs := RequestVoteArgs{
		Term:				rf.currentTerm,
		CandidateIndex:		rf.me,
		LastLogIndex:		rf.lastApplied,
		LastLogTerm:		rf.logTerm[rf.lastApplied],
	}
	rf.mu.Unlock()

	requestVoteReplyList := make([]RequestVoteReply, rf.serverCount)
	for idx := range requestVoteReplyList {
		requestVoteReplyList[idx].Term = -1
	}

	var forceStop int32 = 0

	wg := NewWaitGroupPlus()
	wg.Add(rf.serverCount / 2)

	for idx := range rf.peers {
		if idx == rf.me {
			continue
		}

		go func(serverIdx int) {
			RunUntil(func(args ...interface{})bool {
				return rf.sendRequestVote(args[0].(int), args[1].(*RequestVoteArgs), args[2].(*RequestVoteReply))
			}, func() bool {
				return atomic.LoadInt32(&forceStop) > 0 || !wg.TryWait() || elapseSignature != rf.timerMgr.GetTimerId()
			}, serverIdx, &requestVoteArgs, &requestVoteReplyList[serverIdx])

			rf.mu.Lock()
			defer rf.mu.Unlock()
			if atomic.LoadInt32(&forceStop) > 0 || elapseSignature != rf.timerMgr.GetTimerId() {
				wg.ForceCancelWait()
				return
			}

			if requestVoteReplyList[serverIdx].VoteGranted {
				DPrintf(LogLevelInfo, rf, "Get vote from server %v, Term %v\n", serverIdx, requestVoteReplyList[serverIdx].Term)
				wg.Done()
			} else {
				DPrintf(LogLevelInfo, rf, "Server %v decline vote\n", serverIdx)
			}

			if elapseSignature == rf.timerMgr.GetTimerId() &&
				requestVoteReplyList[serverIdx].Term != -1 && requestVoteReplyList[serverIdx].Term > rf.currentTerm &&
				wg.TryWait() { // to not degrade leader if it has been elected.
				atomic.StoreInt32(&forceStop, 1)
				wg.ForceCancelWait()
				DPrintf(LogLevelInfo, rf, "Found server %v have a bigger term\n", serverIdx)

				rf.currentTerm = requestVoteReplyList[serverIdx].Term
				rf.ChangeState(rf.state, ServerStateFollower)
			}

		}(idx)
	}

	// TODO if this server cannot win election in time and get blocked
	// here should have a termination implementation exit from current method processing.


	wg.Wait()

	rf.mu.Lock()
	defer rf.mu.Unlock()

	DPrintf(LogLevelInfo, rf, "Election end, get %v vote(s) out of %v\n", 1 + wg.Counter(), rf.serverCount)

	if atomic.LoadInt32(&forceStop) > 0 || elapseSignature != rf.timerMgr.GetTimerId(){
		return
	}

	// Win election
	if (wg.Counter() + 1) * 2 > rf.serverCount {
		rf.ChangeState(rf.state, ServerStateLeader)
		rf.SendHeartBeat(elapseSignature)
	}


}
func (rf *Raft) SendHeartBeatOne(server int, elapseSignature int, reply *AppendEntriesReply, lastApplied int) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if elapseSignature != rf.timerMgr.GetTimerId() {
		return
	}

	args := &AppendEntriesArgs{
		Term:				rf.currentTerm,
		LeaderId:			rf.me,
		PrevLogIndex:       lastApplied,
		PrevLogTerm:        rf.logTerm[lastApplied],
		LeaderCommit:       rf.commitIndex,
	}

	// Find log matching point
	for {

		rf.mu.Unlock()
		rf.sendAppendEntriesReliable(server, args, reply, elapseSignature)
		rf.mu.Lock()

		if reply.Success {
			break
		}


		if elapseSignature != rf.timerMgr.GetTimerId() {
			break
		}

		for args.PrevLogIndex > 0 && rf.logTerm[args.PrevLogIndex] == rf.logTerm[args.PrevLogIndex - 1] {
			args.PrevLogIndex--
		}

		args.PrevLogIndex--
		if args.PrevLogIndex < 0 {
			DPrintf(LogLevelWarning, rf, "Log match fail with server %v, Term %v %v %v\n", server, reply.Term, args.PrevLogIndex + 1, args.PrevLogTerm)
			return
		}
		args.PrevLogTerm = rf.logTerm[args.PrevLogIndex]
	}

	// log is already up-to-date.
	if args.PrevLogIndex == lastApplied {
		return
	}

	if elapseSignature != rf.timerMgr.GetTimerId() {
		return
	}

	args.Entries = rf.log[args.PrevLogIndex + 1:lastApplied + 1]

	rf.mu.Unlock()
	rf.sendAppendEntriesReliable(server, args, reply, elapseSignature)
	rf.mu.Lock()
	if !reply.Success {
		DPrintf(LogLevelError, rf, "Send log finally failed.(After log matching)")
	}
}
func (rf *Raft) SendHeartBeat(elapseSignature int) {
	if rf.state != ServerStateLeader {
		DPrintf(LogLevelWarning, rf, "Server state is not leader")
		return
	}

	DPrintf(LogLevelInfo, rf, "Heartbeat start, lastApplied|commitIndex %v|%v\n", rf.lastApplied, rf.commitIndex)

	for idx := range rf.nextIndex {
		rf.nextIndex[idx] = rf.lastApplied + 1
	}

	replyList := make([]AppendEntriesReply, rf.serverCount)
	done 	  := make([]int32, rf.serverCount)
	lastApplied := rf.lastApplied

	wg := NewWaitGroupPlus()
	serverCount := rf.serverCount
	wg.Add(serverCount / 2)

	var replyCount int32 = 0
	for idx := range rf.peers {
		if idx == rf.me {
			continue
		}

		go func(server int, sign int,  reply *AppendEntriesReply) {
			rf.SendHeartBeatOne(server, sign, reply, lastApplied)
			atomic.AddInt32(&replyCount, 1)

			//atomic.StoreInt32(&done[server], 1)
			if atomic.LoadInt32(&replyCount) == int32(serverCount) ||
				sign != rf.timerMgr.GetTimerId() {
				atomic.StoreInt32(&done[server], 1)
				wg.ForceCancelWait()
				atomic.StoreInt32(&done[server], 1)
				return
			}

			if reply.Success {
				rf.mu.Lock()
				rf.matchIndex[server] = replyList[server].CommitIndex
				rf.mu.Unlock()
				atomic.StoreInt32(&done[server], 1)
				wg.Done()

			}
		}(idx, elapseSignature, &replyList[idx])
	}


	// send heartbeat callback
	go func(sign int) {
		wg.Wait()
		rf.mu.Lock()
		defer rf.mu.Unlock()
		if sign != rf.timerMgr.GetTimerId() {
			return
		}

		success := 1
		for idx := range replyList {
			if rf.me == idx {
				continue
			}
			if atomic.LoadInt32(&done[idx]) == 1 && replyList[idx].Success {
				success++
			}
		}

		if success * 2 > rf.serverCount {
			for idx := rf.commitIndex + 1; idx <= lastApplied; idx++ {
				msg := ApplyMsg{true, rf.log[idx], idx}
				rf.mu.Unlock()
				rf.applyCh <- msg
				rf.mu.Lock()
				DPrintf(LogLevelInfo, rf, "ApplyMsg send %v\n", msg)
			}

			rf.commitIndex = lastApplied
			rf.persist()
		}
	}(elapseSignature)
/*
	appendEntriesArgs := AppendEntriesArgs{
		Term:				rf.currentTerm,
		LeaderId:			rf.me,
		PrevLogIndex:       rf.lastApplied,
		PrevLogTerm:        rf.logTerm[rf.lastApplied],
		LeaderCommit:       rf.commitIndex,
	}

	for idx := range rf.peers {
		if idx == rf.me {
			continue
		}

		go func(serverIdx int) {
			rf.sendAppendEntries(serverIdx, &appendEntriesArgs, &replyList[serverIdx])
		}(idx)
	}
*/

	// TODO check ret
	// rf.sendAppendEntriesCallback(appendEntriesReplyList)

}

// make sure only the latest periodic method can execute, the signature should
// identical with timeId in rf.timerMgr
func (rf *Raft) LeaderLoop(elapseSignature int) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if elapseSignature != rf.timerMgr.GetTimerId() {
		DPrintf(LogLevelInfo, rf,"Leader loop mismatch. expect %v, get %v\n", elapseSignature, rf.timerMgr.GetTimerId())
		return
	}

	if rf.state != ServerStateLeader {
		DPrintf(LogLevelWarning, rf, "Current state expect Leader, found %v\n", rf.state)
		return
	}
	DPrintf(LogLevelInfo, rf,"Leader loop start. LastLogTerm|LastLogIndex %v|%v\n", rf.logTerm[rf.lastApplied], rf.lastApplied)
	rf.persist()
	rf.SendHeartBeat(elapseSignature)
}

func (rf *Raft) CandidateLoop(elapseSignature int) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if elapseSignature != rf.timerMgr.GetTimerId() {
		DPrintf(LogLevelInfo, rf,"Candidate loop mismatch. expect %v, get %v\n", elapseSignature, rf.timerMgr.GetTimerId())
		return
	}

	if rf.state != ServerStateCandidate {
		DPrintf(LogLevelWarning, rf, "Current state expect Candidate, found %v\n", rf.state)
		return
	}
	DPrintf(LogLevelInfo, rf, "Candidate loop start. LastLogTerm|LastLogIndex %v|%v\n", rf.logTerm[rf.lastApplied], rf.lastApplied)

	rf.mu.Unlock()
	rf.StartElection(elapseSignature)
	rf.mu.Lock()
}

func (rf *Raft) FollowerLoop(elapseSignature int) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if elapseSignature != rf.timerMgr.GetTimerId() {
		DPrintf(LogLevelInfo, rf,"Follower loop mismatch. expect %v, get %v\n", elapseSignature, rf.timerMgr.GetTimerId())
		return
	}

	if rf.state != ServerStateFollower {
		DPrintf(LogLevelWarning, rf, "Current state expect Follower, found %v\n", rf.state)
		return
	}
	DPrintf(LogLevelInfo, rf, "Follower loop start. LastLogTerm|LastLogIndex %v|%v\n", rf.logTerm[rf.lastApplied], rf.lastApplied)

	rf.ChangeState(rf.state, ServerStateCandidate)
}

func (rf *Raft) ChangeState(oldState int, newState int) {

	DPrintf(LogLevelInfo, rf,"State transfer from %v to %v\n", StateName[oldState], StateName[newState])
	if rf.state != oldState {
		DPrintf(LogLevelWarning, rf,"Server state not match, expect %v get %v\n", oldState, rf.state)
	}

	// TODO init for each state, such as rf.voteFor = -1 while convert to Follower.
	rf.state = newState
	// clean up
	switch oldState {
	case ServerStateLeader:
		// rf.timerMgr.DelTimer(rf.timerId)

	case ServerStateCandidate:
		// rf.timerMgr.DelTimer(rf.timerId)
	case ServerStateFollower:
		// rf.timerMgr.DelTimer(rf.timerId)
	case ServerStateNone:
		// pass

	}

	// set on
	switch newState {
	case ServerStateLeader:
		rf.timerMgr.SetEvent(rf.LeaderLoop, rf.GetLeaderLoopPeriod)
	case ServerStateCandidate:
		if oldState != ServerStateCandidate {
			rf.currentTerm++
		}
		rf.timerMgr.SetEvent(rf.CandidateLoop, rf.GetCandidateLoopPeriod)
	case ServerStateFollower:
		rf.timerMgr.SetEvent(rf.FollowerLoop, rf.GetFollowerLoopPeriod)
	case ServerStateNone:
		// pass
	}
	rf.voteFor = -1
}

func (rf *Raft) ResetCurrentTimer() {
	rf.timerMgr.ResetCurrentEvent()
}

func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {

	InitRandSeed()

	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).
	rf.serverCount = len(peers)
	rf.currentTerm = 0
	rf.voteFor = -1
	rf.applyCh = applyCh
	msg := ApplyMsg{true, 0, 0}

	go func(sendMsg ApplyMsg) { rf.applyCh <- sendMsg }(msg)

	for idx := 0 ; idx < rf.serverCount ; idx++ {
		rf.nextIndex = append(rf.nextIndex, 1)
		rf.matchIndex = append(rf.matchIndex, 0)
	}

	rf.electionTimePeriod = time.Millisecond * 300
	rf.electionTimeWave = time.Millisecond * 100
	rf.heartBeatPeriod = time.Millisecond * 130

	rf.log = append(rf.log, 0)
	rf.logTerm = append(rf.logTerm, 0)


	rf.timerMgr = NewTimerMgr()
	go rf.timerMgr.Schedule()

	// Start from follower.
	rf.state = ServerStateNone

	rf.mu.Lock()

	rf.ChangeState(ServerStateNone, ServerStateFollower)
	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())
	rf.mu.Unlock()

	return rf
}