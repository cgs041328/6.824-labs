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

import (
	"sync"
	"labrpc"
	"time"
	"math/rand"
)
// import "bytes"
// import "labgob"

const(
	Leader = iota
	Candidate
	Follower
)

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

type LogEntry struct{
	Term int
	Index int
	Command interface{}
}

type lockedRand struct {
	mu   sync.Mutex
	rand *rand.Rand
}

func (r *lockedRand) Intn(n int) int {
	r.mu.Lock()
	v := r.rand.Intn(n)
	r.mu.Unlock()
	return v
}

var globalRand = &lockedRand{
	rand: rand.New(rand.NewSource(time.Now().UnixNano())),
}

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	state int
	heartBeatc chan bool
	grantVotec chan bool
	leaderc chan bool
	commitc chan bool
	voteCount int
	heartbeatTimeout int
	electionTimeout  int
	randomizedElectionTimeout int

	//persistent state on all server
	currentTerm int
	votedFor int
	log	[]LogEntry

	//volatile state on all servers
	commitIndex int
	lastApplied int

	//volatile state on leader
	nextIndex []int
	matchIndex []int
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	// Your code here (2A).
	return rf.currentTerm, rf.state == Leader
}


//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
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
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}




//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term int
	CandidateId int
	LastLogIndex int
	LastLogTerm int	
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term int
	VoteGranted bool
}

func (rf *Raft) lastIndex() int {
	return rf.log[len(rf.log) - 1].Index
}
func (rf *Raft) lastTerm() int {
	return rf.log[len(rf.log) - 1].Term
}

func(rf *Raft) isUpToDate(term,index int) bool{
	return term > rf.lastTerm() || (term == rf.lastTerm() && index >= rf.lastIndex())
}
//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	reply.Term = rf.currentTerm
	if(args.Term < rf.currentTerm){
		return
	}
	if (rf.votedFor == -1 || rf.votedFor == args.CandidateId) && rf.isUpToDate(args.LastLogTerm,args.LastLogIndex){
		rf.grantVotec <- true
		rf.currentTerm = args.Term
		rf.state = Follower
		rf.votedFor = args.CandidateId
		reply.VoteGranted = true
	}
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
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if ok {
		if rf.state != Candidate || args.Term != rf.currentTerm {
			return ok
		}
		if reply.Term > rf.currentTerm {
			rf.currentTerm = reply.Term
			rf.state = Follower
			rf.votedFor = -1
			return ok
		}
		if reply.VoteGranted {

			rf.voteCount++
			DPrintf("%d",rf.voteCount)
			if  rf.voteCount > len(rf.peers)/2 {
				rf.leaderc <- true
			}
		}
	}
	return ok
}
func (rf *Raft) broadcastRequestVote() {
	var args RequestVoteArgs
	rf.mu.Lock()
	rf.currentTerm++
	rf.votedFor = rf.me
	rf.voteCount = 1
	args.Term = rf.currentTerm
	args.CandidateId = rf.me
	args.LastLogTerm = rf.lastTerm()
	args.LastLogIndex = rf.lastIndex()
	rf.mu.Unlock()

	for i := range rf.peers {
		if i != rf.me && rf.state == Candidate {
			go func(i int) {
				var reply RequestVoteReply
				rf.sendRequestVote(i, &args, &reply)
			}(i)
		}
	}
}

type AppendEntriesArgs struct {
	// Your data here.
	Term int
	LeaderId int
	PrevLogTerm int
	PrevLogIndex int
	Entries []LogEntry
	LeaderCommit int
}

type AppendEntriesReply struct {
	// Your data here.
	Term int
	Success bool
	NextIndex int
}

func (rf *Raft) AppendEntries(args AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.heartBeatc <- true
	reply.Term = rf.currentTerm

	if args.Term < rf.currentTerm {
		return
	}
	lastLogIndex := rf.lastIndex()

	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.votedFor = -1
		reply.Term = rf.currentTerm
	}	

	if args.PrevLogIndex > lastLogIndex {
		reply.NextIndex = lastLogIndex + 1
		return
	}
	term := rf.lastTerm()
	if term != args.PrevLogTerm {
		var i int
		for i = args.PrevLogIndex - 1; i >= rf.commitIndex; i-- {
			if rf.log[i].Term != term {
				break
			}
		}
		reply.NextIndex = i + 1
		return
	}
	rf.log = rf.log[: args.PrevLogIndex+1]
	rf.log = append(rf.log, args.Entries...)
	reply.Success = true
	reply.NextIndex = rf.lastIndex() + 1
	if args.LeaderCommit > rf.commitIndex {
		last := rf.lastIndex()
		if args.LeaderCommit > last {
			rf.commitIndex = last
		} else {
			rf.commitIndex = args.LeaderCommit
		}
		rf.commitc <- true
	}
	return
}

func (rf *Raft) sendAppendEntries(server int, args AppendEntriesArgs, reply *AppendEntriesReply) bool{
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

func (rf *Raft) broadcastAppendEntries() {
	count := 1
	for i := range rf.peers {
		if i != rf.me && rf.state == Leader {
			go func(i int) {
				nextIndex := rf.nextIndex[i]
					args := AppendEntriesArgs{LeaderId: rf.me, LeaderCommit: rf.commitIndex, Term: rf.currentTerm, PrevLogTerm: -1}
					args.PrevLogIndex = nextIndex - 1
					if args.PrevLogIndex > -1 {
						args.PrevLogTerm = rf.log[args.PrevLogIndex].Term
					}
					if nextIndex < rf.lastIndex() + 1 {
						args.Entries = rf.log[nextIndex:rf.lastIndex() + 1]
					}
				var reply AppendEntriesReply
				ok := rf.sendAppendEntries(i, args, &reply)
				rf.mu.Lock()
				defer rf.mu.Unlock()
				if ok {
					if rf.state != Leader || args.Term != rf.currentTerm {
						return 
					}
			
					if reply.Term > rf.currentTerm {
						rf.currentTerm = reply.Term
						rf.state = Follower
						rf.votedFor = -1
						rf.persist()
						return 
					}
					if reply.Success {
						if len(args.Entries) > 0 {
							rf.nextIndex[i] = args.Entries[len(args.Entries) - 1].Index + 1
							rf.matchIndex[i] = rf.nextIndex[i] - 1
							if count == (len(rf.peers) / 2) {
								rf.commitIndex = rf.lastIndex()
								rf.commitc <- true
							}
							count++
						}
					} else {
						rf.nextIndex[i] = reply.NextIndex
					}
				}
			}(i)
		}
	}
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
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	index = -1
	term = rf.currentTerm
	isLeader = rf.state == Leader
	if isLeader {
		index = rf.lastIndex() + 1
		rf.log = append(rf.log, LogEntry{Term:term,Command:command,Index:index})
		rf.persist()
	}

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
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.state = Follower
	rf.heartbeatTimeout = 100
	rf.electionTimeout = 400
	rf.randomizedElectionTimeout = rf.electionTimeout + globalRand.Intn(rf.electionTimeout)
	rf.votedFor = -1
	rf.heartBeatc = make(chan bool,len(peers))
	rf.grantVotec = make(chan bool,len(peers))
	rf.leaderc = make(chan bool,len(peers))
	rf.log = append(rf.log, LogEntry{Term: 0})

	// Your initialization code here (2A, 2B, 2C).
	go func(){
		for {
			switch rf.state {
			case Follower:
				select{
				case <-rf.heartBeatc:
				case <-rf.grantVotec:
				case <-time.After(time.Duration(rf.randomizedElectionTimeout) * time.Millisecond):
					rf.state = Candidate
				}
			case Candidate:
				go rf.broadcastRequestVote()
				select{
				case <-rf.heartBeatc:
					rf.state = Follower
				case <-time.After(time.Duration(rf.randomizedElectionTimeout) * time.Millisecond):
				case <-rf.leaderc:
					rf.mu.Lock()
					rf.state = Leader
					rf.nextIndex = make([]int,len(rf.peers))
					rf.matchIndex = make([]int,len(rf.peers))
					for i := range rf.peers {
						rf.nextIndex[i] = rf.lastIndex() + 1
					}
					rf.mu.Unlock()
				}
			case Leader:
				rf.broadcastAppendEntries()
				time.Sleep(time.Duration(rf.heartbeatTimeout) * time.Millisecond)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-rf.commitc:
				rf.mu.Lock()
			  commitIndex := rf.commitIndex
				baseIndex := rf.log[0].Index
				for i := rf.lastApplied + 1; i <= commitIndex; i++ {
					msg := ApplyMsg{CommandIndex: i, Command: rf.log[i-baseIndex].Command}
					applyCh <- msg
					rf.lastApplied = i
				}
				rf.mu.Unlock()
			}
		}
	}()
	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())


	return rf
}
