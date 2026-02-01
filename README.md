# SNDV-KV: Blackboard Architecture for LSM Storage

> **Proving that autonomous agents can outperform traditional database algorithms (maybe)**

A research database exploring whether **Blackboard coordination patterns** (common in AI, Distributed, Electrical and analogue automation systems) can make LSM-trees simpler, smarter, and more adaptive than traditional approaches.

---

## The Core Idea 💡

**Traditional LSM databases:**

```text
Write → Lock → WAL → MemTable → Thread Pool → Compaction Scheduler → ...
(Tightly coupled, complex coordination, static algorithms)
```

**Blackboard LSM (this project):**

```text
         BLACKBOARD (Shared State)
                 ↓
    ┌────────────┼────────────┐
    ↓            ↓            ↓
 Ingest       Flush       Compact
 Agent        Agent        Agent
(autonomous) (autonomous) (autonomous)
```

Each agent observes state and acts independently. No callbacks. No thread pools. No coordination hell.

**Hypothesis:** Autonomous agents can adapt to workloads better than hardcoded algorithms.

---

## Why This Matters 🎯

**Problem:** Modern databases use static algorithms

- Compaction triggers: `if L0.size >= 4 { compact() }` ← hardcoded
- Cache policies: LRU ← one size fits all
- Write buffering: Fixed batch sizes ← no adaptation

**This project explores:** What if agents learned and adapted instead?

- ML compaction agent predicts optimal timing
- Adaptive cache agent learns access patterns  
- Workload-aware agents optimize for read vs write heavy loads

**Goal:** Enable storage systems that handle 1B+ token contexts for LLMs by being smarter, not just faster.

---

## Current Status 📊

### Performance

- to be added on a later basis

### Features

- ✅ Write-Ahead Log (durability)
- ✅ LSM-Tree structure (L0 → L1 compaction)
- ✅ Bloom filters (optimized reads)
- ✅ LRU cache (hot key acceleration)
- ✅ Agent-based coordination
- ⚠️ ML compaction (in progress)
- ⚠️ Adaptive caching (planned)

### Production Readiness

**~40%** - Works well, has known issues, actively improving

**Not production-ready yet**, but getting there.  
Built for research first, production second.

---

## Quick Start 🚀

### Install

```bash
git clone https://github.com/thecharge/sndv-kv
cd sndv-kv
go mod tidy
```

### Run

```bash
# Build
go build -o sndv-kv cmd/server/main.go

# Start with safe defaults (durability enabled)
./sndv-kv -config config_safe.json

# Or fast mode (in-memory, no fsync - for testing)
./sndv-kv -config config_fast.json
```

### Use

```bash
# The server prints an admin token on startup
# Copy it and use in requests

# Write
curl -X POST http://localhost:8080/put \
  -H "Authorization: YOUR_TOKEN" \
  -d '{"key": "user:1", "value": "Alice", "ttl": 3600}'

# Read
curl "http://localhost:8080/get?key=user:1" \
  -H "Authorization: YOUR_TOKEN"

# Delete
curl -X POST "http://localhost:8080/delete?key=user:1" \
  -H "Authorization: YOUR_TOKEN"
```

---

## Architecture Deep Dive 🏗️

### The Blackboard Pattern

**Concept from AI:** Multiple expert agents collaborate through shared memory.

**Applied to Storage:**

```go
type Blackboard struct {
    // Shared State
    MemTable     *SwissTable
    ImmutableMem []*SwissTable
    ActiveWAL    *WAL
    SSTables     [][]SSTableMetadata
    
    // Coordination
    Mutex        sync.RWMutex
    FlushCond    *sync.Cond
}
```

**Agents observe and act:**

```go
// Ingest Agent
for batch := range IngestQueue {
    bb.WAL.AppendBatch(batch)
    bb.MemTable.PutBatch(batch)
    
    if bb.MemTable.Size >= threshold {
        bb.FreezeMemTable()  // Signal other agents
    }
}

// Flush Agent  
for {
    wait_for_signal()
    frozen := bb.ImmutableMem[0]
    WriteSSTable(frozen)
    bb.RemoveImmutable()
}

// Compaction Agent
for {
    if bb.SSTables[0].Len() >= trigger {
        CompactL0toL1()
    }
}
```

**No callbacks. No thread pools. Just agents reacting to state.**

---

### Why This Is Different

| Traditional LSM | Blackboard LSM |
|----------------|----------------|
| Thread pools for coordination | Autonomous agents |
| Callback chains | Direct state observation |
| Static algorithms | Adaptive agents (can use ML) |
| Tight coupling | Loose coupling |
| Hard to reason about | Each agent is simple |
| Hard to modify | Swap agents at runtime |

**Example:** Want smarter compaction?  

- Traditional: Rewrite the scheduler, update thread pools, test everything
- Blackboard: Write new agent, deploy alongside old one, A/B test

---

## Research Questions 🔬

This project is exploring:

1. **Can ML agents beat static algorithms?**
   - Predict optimal compaction timing based on workload
   - Expected improvement: 20-40% latency reduction

2. **Do agents enable emergent optimization?**
   - Can agents cooperate without explicit coordination?
   - Example: Flush agent learns compaction agent's patterns

3. **Is Blackboard simpler than traditional approaches?**
   - Measuring: Lines of code, cyclomatic complexity
   - Hypothesis: 30-50% less coordination code

4. **Can this scale to 1B token contexts?**
   - LLMs need massive KV cache storage
   - Agents could: compress, prefetch, evict intelligently

---

## Benchmarks 📈

Benchmarks can be viewed separately in the folder structure.
For now

1. Quick benchmark `python3 quick_bench.py`  script is in place in order for you to see and get the feelign of the engine
2. A comperhensive multi stage build and test `python bench_orchestrator.py` is in place so you can see the full suite, tests and benches done
When I habve time I will add more detailed explanations of the suites and their metrics.

Here is the last quick benchmark I ran:
evidence for the last quick bench in the results.json in th eroot of the repository (Have that it may vary from PC to PC the bench orchestrator uses much more evidence and integration test data - but is docker and python bound as well as it will require much more time to run in future a seprate folder with version to version benchmarks and configurations will be in place - but for now I do not have time to polish and will wait until there is a time for production or someone decides to implement that in MR)

```bash
 python .\quick_bench.py
Building...
Starting server...

Single ops (200)...
  710 TPS

Batch ops (1000)...
  43,084 TPS

========================================
Single:        710 TPS
Batch:      43,084 TPS
========================================
```

**We're slower than production systems.** That's expected for:

- Research codebase vs production
- Go vs C/C++
- Novel architecture vs proven
- Solo developer vs teams

**The goal isn't to beat RocksDB in performance.**  
**The goal is to prove agents can be smarter.**

---

## Why Go? (Not Rust/C++) 🤔

**Common question:** "Why not use Rust/C++ for a database?"

**Honest answer:** Because I want to learn Go while proving the hypothesis.

**Practical reasons:**

1. **Fast iteration** - Go compiles in seconds, not minutes
2. **Simple concurrency** - Goroutines perfect for agent model
3. **Good enough performance** - 7K TPS is plenty for research
4. **Easy to read** - Research code should be understandable
5. **Rich ecosystem** - Good libraries for benchmarking, testing

**Will I switch to Rust/C++ later?**  
Maybe. If the research proves Blackboard works, a production rewrite makes sense.

**Right now:** I'm riding the Go wave and learning while building.

---

## Contributing 🤝

### For Researchers

Interested in agent-based storage systems? Let's collaborate!

**Open research questions:**

- How to train ML compaction agents?
- What metrics predict optimal compaction timing?
- Can agents learn workload patterns?

<!-- **See:** [Research Roadmap](docs/RESEARCH.md) -->

### For Engineers  

Want to learn LSM internals? Welcome!

**Good first issues:**

- Fix known bugs (see Issues)
- Add tests for critical paths
- Implement missing features

<!-- **See:** [Contributing Guide](CONTRIBUTING.md) -->

### For Skeptics

Think Blackboard is overkill? Prove me wrong!

**I'm looking for:**

- Benchmark comparisons
- Architecture critiques  
- Performance bottleneck analysis

**See:** [Discussions](https://github.com/thecharge/sndv-kv/discussions)

---

## Roadmap 🗺️

### Phase 1: Foundation (Now → Month 3)

- [x] Basic LSM structure
- [x] WAL with crash recovery
- [x] Agent-based coordination
- [ ] Fix critical bugs
- [ ] Comprehensive test suite
- [ ] Honest benchmarks published

### Phase 2: Intelligence (Month 3-6)

- [ ] ML compaction agent
- [ ] Adaptive cache agent
- [ ] Workload prediction
- [ ] A/B testing framework
- [ ] Paper: "Blackboard Architectures for LSM-Trees"

### Phase 3: Scale (Month 6-12)

- [ ] 100K+ TPS sustained
- [ ] 1B token context support
- [ ] Agent marketplace (swap at runtime)
- [ ] Production deployment case study

### Phase 4: Ecosystem (Year 2)

- [ ] Rust rewrite (if research proves it)
- [ ] Multi-language bindings
- [ ] Cloud-native deployment
- [ ] Conference talks & papers

---

## Known Issues ⚠️

Being honest about limitations:

1. **Performance:** 15-20x slower than RocksDB (expected, being addressed)
2. **Testing:** Only ~30% code coverage (improving)
3. **Production:** Not ready for critical workloads yet
4. **Documentation:** Some internals not fully documented
5. **Compaction:** Only L0→L1, no multi-level yet

**These are features, not bugs** - they're the research agenda!

---

## Philosophy 📖

### This Project Believes

**Simple > Complex**  

- Small autonomous agents beat complex coordinators
- Loose coupling beats tight coupling
- Observable state beats callback chains

**Adaptive > Static**

- ML agents beat hardcoded thresholds
- Learning systems beat fixed algorithms
- Workload-aware beats one-size-fits-all

**Research > Perfection**

- Ship experiments, measure results
- Fail fast, learn faster
- Prove concepts, then optimize

**Honesty > Marketing**

- Accurate benchmarks, not inflated numbers
- Known issues visible, not hidden
- Research progress tracked publicly

---

## FAQ ❓

**Q: Why Blackboard for databases?**  
A: 8 years of distributed systems convinced me tight coupling is the enemy. Blackboard enables loose coupling at scale.

**Q: When will it be production-ready?**  
A: When it proves agents beat static algorithms. Performance comes after proof.

**Q: Why not just use RocksDB?**  
A: RocksDB is amazing. This explores whether we can be *smarter*, not just faster.

**Q: What about consistency/ACID?**  
A: Single-node strong consistency. Distributed ACID is future work.

**Q: Can I use this for real projects?**  
A: For learning/research: yes. For production: wait for v1.0.

**Q: How can I help?**  
A: Research collaboration, code review, honest feedback - all welcome!

---

## License 📄

MIT License - Use freely, cite generously.

If you build something cool with this, let me know!  
If you publish research using this, please cite.

---

## Contact 📬

**Creator:** Radoslav Sandov  
**Goal:** Prove Blackboard > Traditional LSM  
**Status:** Actively researching, openly sharing

- GitHub: [@thecharge](https://github.com/thecharge)
- Twitter: [@Radoslav_Sandov](https://x.com/Radoslav_Sandov)  
- Email: [Mail](thecharge@gmail.com)
<!-- - Discussions: [GitHub Discussions](https://github.com/thecharge/sndv-kv/discussions) -->

---

## Acknowledgments 🙏

- **LevelDB/RocksDB** - Reference implementation inspiration
- **BadgerDB** - Go LSM-tree design patterns
- **Blackboard Systems** - AI coordination patterns from the 1980s
- **Claude/Gemini/GPT** - AI pair programming for faster iteration
- **Everyone who reviewed this** - Brutal feedback makes better software

---

**Built with curiosity. Accelerated with AI. Driven by ego to prove a point.**

*If you're reading this, you're early. Come build the future of storage systems with me.* 🚀

---

## Stats

![GitHub stars](https://img.shields.io/github/stars/thecharge/sndv-kv?style=social)
![GitHub forks](https://img.shields.io/github/forks/thecharge/sndv-kv?style=social)
![Lines of code](https://img.shields.io/tokei/lines/github/thecharge/sndv-kv)
![Go version](https://img.shields.io/github/go-mod/go-version/thecharge/sndv-kv)
![License](https://img.shields.io/github/license/thecharge/sndv-kv)
