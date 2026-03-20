OUT ?= out
HEADER ?= $(OUT)/header
RESULTS ?= $(OUT)/results
CPU_OUT ?= 
MEM_OUT ?= 
COUNT ?= 10
Richglob ?= $(OUT)/richglob
Doublestar ?= $(OUT)/doublestar
Std ?= $(OUT)/std
PROFILE_FLAGS := $(if $(strip $(CPU_OUT)),-cpuprofile $(CPU_OUT)) $(if $(strip $(MEM_OUT)),-memprofile $(MEM_OUT))

bench:
	@mkdir -p $(OUTPUT_DIRS) 2>/dev/null || true
	@rm -f $(OUTPUTS)

	@go test -count=$(COUNT) -bench=. -benchmem $(PROFILE_FLAGS) | tee $(RESULTS)
	@grep -E '^(goos|goarch|pkg|cpu):' $(RESULTS) > $(HEADER)

	@cat $(HEADER) > $(Richglob)
	@grep '^BenchmarkRichglob/' $(RESULTS) | sed 's/BenchmarkRichglob\//Benchmark/' >> $(Richglob)
	@echo "PASS" >> $(Richglob)

	@cat $(HEADER) > $(Doublestar)
	@grep '^BenchmarkDoublestar/' $(RESULTS) | sed 's/BenchmarkDoublestar\//Benchmark/' >> $(Doublestar)
	@echo "PASS" >> $(Doublestar)

	@cat $(HEADER) > $(Std)
	@grep '^BenchmarkStd/' $(RESULTS) | sed 's/BenchmarkStd\//Benchmark/' >> $(Std)
	@echo "PASS" >> $(Std)

