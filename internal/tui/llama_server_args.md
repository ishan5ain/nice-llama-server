# llama-server args catalog

Source: extracted from llama.cpp tools/server README. Keep this file tracked; `.devlocal` is local scratch only.

| Argument |
| -------- |
| `-h, --help, --usage` |
| `--version` |
| `--license` |
| `-cl, --cache-list` |
| `--completion-bash` |
| `-t, --threads N` |
| `-tb, --threads-batch N` |
| `-C, --cpu-mask M` |
| `-Cr, --cpu-range lo-hi` |
| `--cpu-strict <0\|1>` |
| `--prio N` |
| `--poll <0...100>` |
| `-Cb, --cpu-mask-batch M` |
| `-Crb, --cpu-range-batch lo-hi` |
| `--cpu-strict-batch <0\|1>` |
| `--prio-batch N` |
| `--poll-batch <0\|1>` |
| `-c, --ctx-size N` |
| `-n, --predict, --n-predict N` |
| `-b, --batch-size N` |
| `-ub, --ubatch-size N` |
| `--keep N` |
| `--swa-full` |
| `-fa, --flash-attn [on\|off\|auto]` |
| `--perf, --no-perf` |
| `-e, --escape, --no-escape` |
| `--rope-scaling {none,linear,yarn}` |
| `--rope-scale N` |
| `--rope-freq-base N` |
| `--rope-freq-scale N` |
| `--yarn-orig-ctx N` |
| `--yarn-ext-factor N` |
| `--yarn-attn-factor N` |
| `--yarn-beta-slow N` |
| `--yarn-beta-fast N` |
| `-kvo, --kv-offload, -nkvo, --no-kv-offload` |
| `--repack, -nr, --no-repack` |
| `--no-host` |
| `-ctk, --cache-type-k TYPE` |
| `-ctv, --cache-type-v TYPE` |
| `-dt, --defrag-thold N` |
| `--mlock` |
| `--mmap, --no-mmap` |
| `-dio, --direct-io, -ndio, --no-direct-io` |
| `--numa TYPE` |
| `-dev, --device <dev1,dev2,..>` |
| `--list-devices` |
| `-ot, --override-tensor <tensor name pattern>=<buffer type>,...` |
| `-cmoe, --cpu-moe` |
| `-ncmoe, --n-cpu-moe N` |
| `-ngl, --gpu-layers, --n-gpu-layers N` |
| `-sm, --split-mode {none,layer,row}` |
| `-ts, --tensor-split N0,N1,N2,...` |
| `-mg, --main-gpu INDEX` |
| `-fit, --fit [on\|off]` |
| `-fitt, --fit-target MiB0,MiB1,MiB2,...` |
| `-fitc, --fit-ctx N` |
| `--check-tensors` |
| `--override-kv KEY=TYPE:VALUE,...` |
| `--op-offload, --no-op-offload` |
| `--lora FNAME` |
| `--lora-scaled FNAME:SCALE,...` |
| `--control-vector FNAME` |
| `--control-vector-scaled FNAME:SCALE,...` |
| `--control-vector-layer-range START END` |
| `-m, --model FNAME` |
| `-mu, --model-url MODEL_URL` |
| `-dr, --docker-repo [<repo>/]<model>[:quant]` |
| `-hf, -hfr, --hf-repo <user>/<model>[:quant]` |
| `-hfd, -hfrd, --hf-repo-draft <user>/<model>[:quant]` |
| `-hff, --hf-file FILE` |
| `-hfv, -hfrv, --hf-repo-v <user>/<model>[:quant]` |
| `-hffv, --hf-file-v FILE` |
| `-hft, --hf-token TOKEN` |
| `--log-disable` |
| `--log-file FNAME` |
| `--log-colors [on\|off\|auto]` |
| `-v, --verbose, --log-verbose` |
| `--offline` |
| `-lv, --verbosity, --log-verbosity N` |
| `--log-prefix` |
| `--log-timestamps` |
| `-ctkd, --cache-type-k-draft TYPE` |
| `-ctvd, --cache-type-v-draft TYPE` |
| `--samplers SAMPLERS` |
| `-s, --seed SEED` |
| `--sampler-seq, --sampling-seq SEQUENCE` |
| `--ignore-eos` |
| `--temp, --temperature N` |
| `--top-k N` |
| `--top-p N` |
| `--min-p N` |
| `--top-nsigma, --top-n-sigma N` |
| `--xtc-probability N` |
| `--xtc-threshold N` |
| `--typical, --typical-p N` |
| `--repeat-last-n N` |
| `--repeat-penalty N` |
| `--presence-penalty N` |
| `--frequency-penalty N` |
| `--dry-multiplier N` |
| `--dry-base N` |
| `--dry-allowed-length N` |
| `--dry-penalty-last-n N` |
| `--dry-sequence-breaker STRING` |
| `--adaptive-target N` |
| `--adaptive-decay N` |
| `--dynatemp-range N` |
| `--dynatemp-exp N` |
| `--mirostat N` |
| `--mirostat-lr N` |
| `--mirostat-ent N` |
| `-l, --logit-bias TOKEN_ID(+/-)BIAS` |
| `--grammar GRAMMAR` |
| `--grammar-file FNAME` |
| `-j, --json-schema SCHEMA` |
| `-jf, --json-schema-file FILE` |
| `-bs, --backend-sampling` |
| `-lcs, --lookup-cache-static FNAME` |
| `-lcd, --lookup-cache-dynamic FNAME` |
| `-ctxcp, --ctx-checkpoints, --swa-checkpoints N` |
| `-cpent, --checkpoint-every-n-tokens N` |
| `-cram, --cache-ram N` |
| `-kvu, --kv-unified, -no-kvu, --no-kv-unified` |
| `--clear-idle, --no-clear-idle` |
| `--context-shift, --no-context-shift` |
| `-r, --reverse-prompt PROMPT` |
| `-sp, --special` |
| `--warmup, --no-warmup` |
| `--spm-infill` |
| `--pooling {none,mean,cls,last,rank}` |
| `-np, --parallel N` |
| `-cb, --cont-batching, -nocb, --no-cont-batching` |
| `-mm, --mmproj FILE` |
| `-mmu, --mmproj-url URL` |
| `--mmproj-auto, --no-mmproj, --no-mmproj-auto` |
| `--mmproj-offload, --no-mmproj-offload` |
| `--image-min-tokens N` |
| `--image-max-tokens N` |
| `-otd, --override-tensor-draft <tensor name pattern>=<buffer type>,...` |
| `-cmoed, --cpu-moe-draft` |
| `-ncmoed, --n-cpu-moe-draft N` |
| `-a, --alias STRING` |
| `--tags STRING` |
| `--host HOST` |
| `--port PORT` |
| `--reuse-port` |
| `--path PATH` |
| `--api-prefix PREFIX` |
| `--webui-config JSON` |
| `--webui-config-file PATH` |
| `--webui-mcp-proxy, --no-webui-mcp-proxy` |
| `--tools TOOL1,TOOL2,...` |
| `--webui, --no-webui` |
| `--embedding, --embeddings` |
| `--rerank, --reranking` |
| `--api-key KEY` |
| `--api-key-file FNAME` |
| `--ssl-key-file FNAME` |
| `--ssl-cert-file FNAME` |
| `--chat-template-kwargs STRING` |
| `-to, --timeout N` |
| `--threads-http N` |
| `--cache-prompt, --no-cache-prompt` |
| `--cache-reuse N` |
| `--metrics` |
| `--props` |
| `--slots, --no-slots` |
| `--slot-save-path PATH` |
| `--media-path PATH` |
| `--models-dir PATH` |
| `--models-preset PATH` |
| `--models-max N` |
| `--models-autoload, --no-models-autoload` |
| `--jinja, --no-jinja` |
| `--reasoning-format FORMAT` |
| `-rea, --reasoning [on\|off\|auto]` |
| `--reasoning-budget N` |
| `--reasoning-budget-message MESSAGE` |
| `--chat-template JINJA_TEMPLATE` |
| `--chat-template-file JINJA_TEMPLATE_FILE` |
| `--skip-chat-parsing, --no-skip-chat-parsing` |
| `--prefill-assistant, --no-prefill-assistant` |
| `-sps, --slot-prompt-similarity SIMILARITY` |
| `--lora-init-without-apply` |
| `--sleep-idle-seconds SECONDS` |
| `-td, --threads-draft N` |
| `-tbd, --threads-batch-draft N` |
| `--draft, --draft-n, --draft-max N` |
| `--draft-min, --draft-n-min N` |
| `--draft-p-min P` |
| `-cd, --ctx-size-draft N` |
| `-devd, --device-draft <dev1,dev2,..>` |
| `-ngld, --gpu-layers-draft, --n-gpu-layers-draft N` |
| `-md, --model-draft FNAME` |
| `--spec-replace TARGET DRAFT` |
| `--spec-type [none\|ngram-cache\|ngram-simple\|ngram-map-k\|ngram-map-k4v\|ngram-mod]` |
| `--spec-ngram-size-n N` |
| `--spec-ngram-size-m N` |
| `--spec-ngram-min-hits N` |
| `-mv, --model-vocoder FNAME` |
| `--tts-use-guide-tokens` |
| `--embd-gemma-default` |
| `--fim-qwen-1.5b-default` |
| `--fim-qwen-3b-default` |
| `--fim-qwen-7b-default` |
| `--fim-qwen-7b-spec` |
| `--fim-qwen-14b-spec` |
| `--fim-qwen-30b-default` |
| `--gpt-oss-20b-default` |
| `--gpt-oss-120b-default` |
| `--vision-gemma-4b-default` |
| `--vision-gemma-12b-default` |
