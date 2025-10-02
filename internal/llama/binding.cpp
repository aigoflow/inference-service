#include "binding.h"
#include "llama.h"
#include <cstring>
#include <string>
#include <vector>

extern "C" {

void* load_model(const char *fname, int n_ctx, int n_threads, int n_gpu_layers, bool use_mmap, bool use_mlock) {
    llama_model_params model_params = llama_model_default_params();
    model_params.n_gpu_layers = n_gpu_layers;
    model_params.use_mmap = use_mmap;
    model_params.use_mlock = use_mlock;
    
    llama_model* model = llama_model_load_from_file(fname, model_params);
    return (void*)model;
}

void* load_embedding_model(const char *fname, int n_ctx, int n_threads, int n_gpu_layers, bool use_mmap, bool use_mlock) {
    llama_model_params model_params = llama_model_default_params();
    model_params.n_gpu_layers = n_gpu_layers;
    model_params.use_mmap = use_mmap;
    model_params.use_mlock = use_mlock;
    
    llama_model* model = llama_model_load_from_file(fname, model_params);
    return (void*)model;
}

void free_model(void* model) {
    if (model) {
        llama_model_free((llama_model*)model);
    }
}

void* new_context(void* model, int n_ctx, int n_threads) {
    if (!model) return nullptr;
    
    llama_context_params ctx_params = llama_context_default_params();
    ctx_params.n_ctx = n_ctx > 0 ? n_ctx : 4096;
    ctx_params.n_threads = n_threads > 0 ? n_threads : 8;
    
    llama_context* ctx = llama_init_from_model((llama_model*)model, ctx_params);
    return (void*)ctx;
}

void* new_embedding_context(void* model, int n_ctx, int n_threads) {
    if (!model) return nullptr;
    
    llama_context_params ctx_params = llama_context_default_params();
    ctx_params.n_ctx = n_ctx > 0 ? n_ctx : 4096;
    ctx_params.n_threads = n_threads > 0 ? n_threads : 8;
    ctx_params.pooling_type = LLAMA_POOLING_TYPE_MEAN;  // Enable pooling for embeddings
    
    // Critical settings for embedding models (copied from working example)
    ctx_params.n_batch = ctx_params.n_ctx;    // Set batch size to context size
    ctx_params.n_ubatch = ctx_params.n_batch; // For non-causal models
    ctx_params.embeddings = true;             // Enable embeddings
    
    llama_context* ctx = llama_init_from_model((llama_model*)model, ctx_params);
    return (void*)ctx;
}

void free_context(void* ctx) {
    if (ctx) {
        llama_free((llama_context*)ctx);
    }
}

void clear_context(void* ctx) {
    // TODO: Find correct cache clearing function in current API
    // For now, context reuse is acceptable for testing
}

int llama_predict(void* ctx, const char* prompt, char* result, int result_size,
                  int max_tokens, float temperature, float top_p, int top_k,
                  float repeat_penalty, int repeat_last_n, bool use_penalty) {
    if (!ctx || !prompt || !result) return -1;
    
    llama_context* context = (llama_context*)ctx;
    const llama_model* model = llama_get_model(context);
    const llama_vocab* vocab = llama_model_get_vocab(model);
    
    // Tokenize prompt exactly like simple.cpp
    const int n_prompt = -llama_tokenize(vocab, prompt, strlen(prompt), NULL, 0, true, true);
    if (n_prompt <= 0) return -1;
    
    std::vector<llama_token> prompt_tokens(n_prompt);
    llama_tokenize(vocab, prompt, strlen(prompt), prompt_tokens.data(), prompt_tokens.size(), true, true);
    
    // Set up sampling chain (use greedy for now to avoid complexity)
    auto sparams = llama_sampler_chain_default_params();
    sparams.no_perf = false;
    llama_sampler* smpl = llama_sampler_chain_init(sparams);
    llama_sampler_chain_add(smpl, llama_sampler_init_greedy());
    
    std::string generated_text;
    int tokens_generated = 0;
    
    // Main generation loop with chunked prompt processing
    int n_pos = 0;
    const int BATCH_SIZE = 512;  // Process in chunks to avoid memory issues
    
    // Process prompt in chunks first
    printf("[DEBUG] Processing prompt with %d tokens in chunks of %d\n", n_prompt, BATCH_SIZE);
    fflush(stdout);
    
    for (int chunk_start = 0; chunk_start < n_prompt; chunk_start += BATCH_SIZE) {
        int chunk_size = std::min(BATCH_SIZE, n_prompt - chunk_start);
        llama_batch chunk_batch = llama_batch_get_one(prompt_tokens.data() + chunk_start, chunk_size);
        
        printf("[DEBUG] Processing chunk %d-%d (%d tokens)\n", chunk_start, chunk_start + chunk_size - 1, chunk_size);
        fflush(stdout);
        
        int decode_result = llama_decode(context, chunk_batch);
        printf("[DEBUG] Chunk decode result: %d\n", decode_result);
        fflush(stdout);
        
        if (decode_result) {
            printf("[DEBUG] Chunk decode failed, aborting\n");
            fflush(stdout);
            return -1;
        }
        n_pos += chunk_size;
    }
    
    printf("[DEBUG] Prompt processing complete, starting generation\n");
    fflush(stdout);
    
    // Now start generation loop
    for (int i = 0; i < max_tokens; ) {
        // Sample next token
        printf("[DEBUG] About to sample next token at position %d\n", n_pos);
        fflush(stdout);
        llama_token new_token_id = llama_sampler_sample(smpl, context, -1);
        printf("[DEBUG] Sampled token: %d\n", new_token_id);
        fflush(stdout);
        
        // Check for end of generation
        if (llama_vocab_is_eog(vocab, new_token_id)) {
            printf("[DEBUG] End of generation token detected\n");
            fflush(stdout);
            break;
        }
        
        // Convert token to text
        printf("[DEBUG] About to convert token to text\n");
        fflush(stdout);
        char buf[128];
        int n = llama_token_to_piece(vocab, new_token_id, buf, sizeof(buf), 0, true);
        printf("[DEBUG] Token conversion result: %d bytes\n", n);
        fflush(stdout);
        if (n > 0) {
            generated_text.append(buf, n);
            tokens_generated++;
            printf("[DEBUG] Generated token %d, total length: %zu\n", tokens_generated, generated_text.length());
            fflush(stdout);
        }
        
        // Accept token for sampler state management
        llama_sampler_accept(smpl, new_token_id);
        
        // Prepare next batch with the new token and decode it
        printf("[DEBUG] Preparing batch for next token: %d\n", new_token_id);
        fflush(stdout);
        llama_batch next_batch = llama_batch_get_one(&new_token_id, 1);
        int decode_result = llama_decode(context, next_batch);
        printf("[DEBUG] Next token decode result: %d\n", decode_result);
        fflush(stdout);
        
        if (decode_result) {
            printf("[DEBUG] Next token decode failed, breaking generation\n");
            fflush(stdout);
            break;
        }
        
        n_pos += 1;
        i++;
    }
    
    llama_sampler_free(smpl);
    
    // Copy result
    size_t len = std::min((size_t)(result_size - 1), generated_text.length());
    strncpy(result, generated_text.c_str(), len);
    result[len] = '\0';
    
    return tokens_generated;
}

int llama_predict_with_grammar(void* ctx, const char* prompt, char* result, int result_size,
                              int max_tokens, float temperature, float top_p, int top_k,
                              float repeat_penalty, int repeat_last_n, 
                              const char* grammar_str) {
    if (!ctx || !prompt || !result) return -1;
    
    llama_context* context = (llama_context*)ctx;
    const llama_model* model = llama_get_model(context);
    const llama_vocab* vocab = llama_model_get_vocab(model);
    
    // Tokenize prompt
    const int n_prompt = -llama_tokenize(vocab, prompt, strlen(prompt), NULL, 0, true, true);
    if (n_prompt <= 0) return -1;
    
    std::vector<llama_token> prompt_tokens(n_prompt);
    llama_tokenize(vocab, prompt, strlen(prompt), prompt_tokens.data(), prompt_tokens.size(), true, true);
    
    // Evaluate prompt
    llama_batch batch = llama_batch_get_one(prompt_tokens.data(), prompt_tokens.size());
    if (llama_decode(context, batch) != 0) {
        return -1;
    }
    
    // Set up sampling chain with grammar
    auto sparams = llama_sampler_chain_default_params();
    sparams.no_perf = false;
    
    llama_sampler* smpl = llama_sampler_chain_init(sparams);
    
    // Add grammar sampler if grammar provided (most restrictive, add first)
    if (grammar_str && strlen(grammar_str) > 0) {
        // Debug: print grammar being used
        printf("Using grammar (length %zu): %.100s...\n", strlen(grammar_str), grammar_str);
        
        llama_sampler* grammar_sampler = llama_sampler_init_grammar(vocab, grammar_str, "root");
        if (grammar_sampler) {
            printf("Grammar sampler created successfully\n");
            llama_sampler_chain_add(smpl, grammar_sampler);
        } else {
            printf("ERROR: Grammar sampler creation failed\n");
        }
    }
    
    // Add other samplers
    llama_sampler_chain_add(smpl, llama_sampler_init_top_k(top_k));
    llama_sampler_chain_add(smpl, llama_sampler_init_top_p(top_p, 1));
    llama_sampler_chain_add(smpl, llama_sampler_init_temp(temperature));
    
    std::string generated_text;
    int tokens_generated = 0;
    
    // Generation loop with timeout protection for grammar issues
    int generation_attempts = 0;
    const int MAX_GENERATION_ATTEMPTS = max_tokens * 2; // Safety limit
    
    for (int n_pos = 0; n_pos < max_tokens && n_pos + batch.n_tokens < n_prompt + max_tokens && generation_attempts < MAX_GENERATION_ATTEMPTS; ) {
        if (llama_decode(context, batch)) {
            printf("ERROR: llama_decode failed\n");
            break;
        }
        
        n_pos += batch.n_tokens;
        generation_attempts++;
        
        // Add protection against infinite loops in grammar sampling
        printf("Sampling token %d/%d...\n", tokens_generated + 1, max_tokens);
        
        // Try grammar sampling with error protection
        llama_token new_token_id;
        try {
            new_token_id = llama_sampler_sample(smpl, context, -1);
            printf("Sampled token: %d\n", new_token_id);
        } catch (...) {
            printf("Grammar sampling failed, falling back to greedy\n");
            // Fallback: create simple greedy sampler
            llama_sampler* fallback_smpl = llama_sampler_chain_init(llama_sampler_chain_default_params());
            llama_sampler_chain_add(fallback_smpl, llama_sampler_init_greedy());
            new_token_id = llama_sampler_sample(fallback_smpl, context, -1);
            llama_sampler_free(fallback_smpl);
            printf("Fallback sampled token: %d\n", new_token_id);
        }
        
        if (llama_vocab_is_eog(vocab, new_token_id)) {
            printf("End of generation token detected\n");
            break;
        }
        
        char buf[128];
        int n = llama_token_to_piece(vocab, new_token_id, buf, sizeof(buf), 0, true);
        if (n > 0) {
            generated_text.append(buf, n);
            tokens_generated++;
            printf("Generated token %d: '%.*s'\n", tokens_generated, n, buf);
        }
        
        // CRITICAL: Accept the token for grammar state management
        llama_sampler_accept(smpl, new_token_id);
        
        batch = llama_batch_get_one(&new_token_id, 1);
    }
    
    if (generation_attempts >= MAX_GENERATION_ATTEMPTS) {
        printf("WARNING: Generation stopped due to attempt limit (possible infinite loop)\n");
    }
    
    llama_sampler_free(smpl);
    
    // Copy result
    size_t len = std::min((size_t)(result_size - 1), generated_text.length());
    strncpy(result, generated_text.c_str(), len);
    result[len] = '\0';
    
    return tokens_generated;
}

int count_tokens(void* ctx, const char* text) {
    if (!ctx || !text) return 0;
    
    llama_context* context = (llama_context*)ctx;
    const llama_model* model = llama_get_model(context);
    const llama_vocab* vocab = llama_model_get_vocab(model);
    
    // Get token count using two-step approach
    const int n_tokens = -llama_tokenize(vocab, text, strlen(text), NULL, 0, true, true);
    return n_tokens > 0 ? n_tokens : 0;
}

int get_context_size(void* model) {
    if (!model) return 0;
    return llama_model_n_ctx_train((llama_model*)model);
}

bool has_gpu_support() {
#ifdef GGML_USE_METAL
    return true;
#elif defined(GGML_USE_CUDA)
    return true;
#elif defined(GGML_USE_VULKAN)
    return true;
#else
    return false;
#endif
}

int llama_embedding(void* ctx, const char* text, float* embeddings, int max_embeddings) {
    if (!ctx || !text || !embeddings) return -1;
    
    llama_context* context = (llama_context*)ctx;
    const llama_model* model = llama_get_model(context);
    const llama_vocab* vocab = llama_model_get_vocab(model);
    
    // Tokenize input text
    const int n_tokens = -llama_tokenize(vocab, text, strlen(text), NULL, 0, true, true);
    if (n_tokens <= 0) return -1;
    
    std::vector<llama_token> tokens(n_tokens);
    llama_tokenize(vocab, text, strlen(text), tokens.data(), tokens.size(), true, true);
    
    // Create batch for embedding with proper setup
    llama_batch batch = llama_batch_init(tokens.size(), 0, 1);
    
    // Add tokens to batch with seq_id = 0 and enable logits for embedding extraction
    for (size_t i = 0; i < tokens.size(); i++) {
        batch.token[i] = tokens[i];
        batch.pos[i] = i;
        batch.n_seq_id[i] = 1;
        batch.seq_id[i][0] = 0;  // sequence ID 0
        batch.logits[i] = true;  // Enable logits for embedding
    }
    batch.n_tokens = tokens.size();
    
    // Clear previous kv_cache values (irrelevant for embeddings)
    llama_memory_clear(llama_get_memory(context), true);
    
    // Process tokens for embedding (use decode for embeddings)
    if (llama_decode(context, batch) < 0) {
        llama_batch_free(batch);
        return -1; // Failed to decode
    }
    
    // Get embeddings from the model
    int n_embd = llama_model_n_embd(model);
    if (n_embd > max_embeddings) {
        n_embd = max_embeddings;
    }
    
    // Get sequence embeddings (pooled) - more appropriate for text embeddings
    const float* model_embeddings = llama_get_embeddings_seq(context, 0);
    if (!model_embeddings) {
        // Fallback to token embeddings from last token
        model_embeddings = llama_get_embeddings_ith(context, tokens.size() - 1);
    }
    
    if (!model_embeddings) {
        llama_batch_free(batch);
        return -1; // No embeddings available
    }
    
    // Copy embeddings to output buffer
    memcpy(embeddings, model_embeddings, n_embd * sizeof(float));
    
    llama_batch_free(batch);
    
    return n_embd;
}

int get_embedding_size(void* model) {
    if (!model) return 0;
    return llama_model_n_embd((llama_model*)model);
}

// Model introspection functions
const char* get_model_architecture(void* model) {
    if (!model) return "unknown";
    
    const llama_model* m = (const llama_model*)model;
    
    // Try to get architecture from model metadata using buffer
    static char arch_buf[64];
    int32_t result = llama_model_meta_val_str(m, "general.architecture", arch_buf, sizeof(arch_buf));
    
    return (result > 0) ? arch_buf : "llama";  // Default to llama if not found
}

const char* get_model_name(void* model) {
    if (!model) return "unknown";
    
    const llama_model* m = (const llama_model*)model;
    
    static char name_buf[128];
    int32_t result = llama_model_meta_val_str(m, "general.name", name_buf, sizeof(name_buf));
    
    return (result > 0) ? name_buf : "unnamed";
}

int get_model_parameter_count(void* model) {
    if (!model) return 0;
    
    const llama_model* m = (const llama_model*)model;
    
    // Try to get parameter count from metadata
    // This might not always be available in GGUF
    return llama_model_n_params(m);
}

const char* get_model_quantization(void* model) {
    if (!model) return "unknown";
    
    const llama_model* m = (const llama_model*)model;
    
    static char quant_buf[32];
    int32_t result = llama_model_meta_val_str(m, "general.quantization_version", quant_buf, sizeof(quant_buf));
    
    return (result > 0) ? quant_buf : "fp16";  // Default assumption
}

const char* get_model_family(void* model) {
    if (!model) return "unknown";
    
    const llama_model* m = (const llama_model*)model;
    
    static char family_buf[64];
    int32_t result = llama_model_meta_val_str(m, "general.family", family_buf, sizeof(family_buf));
    
    if (result > 0) return family_buf;
    
    // Fallback to architecture-based family detection
    const char* arch = get_model_architecture(model);
    if (strstr(arch, "llama")) return "llama";
    if (strstr(arch, "gemma")) return "gemma";
    if (strstr(arch, "qwen")) return "qwen";
    if (strstr(arch, "phi")) return "phi";
    
    return "unknown";
}

bool model_supports_images(void* model) {
    if (!model) return false;
    
    const llama_model* m = (const llama_model*)model;
    
    // Check if model has vision/image processing capabilities
    // This is determined by checking for vision-specific layers or metadata
    const char* arch = get_model_architecture(model);
    
    // Common vision-enabled architectures
    return strstr(arch, "llava") != nullptr ||
           strstr(arch, "clip") != nullptr ||
           strstr(arch, "vision") != nullptr ||
           strstr(arch, "multimodal") != nullptr;
}

bool model_supports_audio(void* model) {
    if (!model) return false;
    
    const llama_model* m = (const llama_model*)model;
    const char* arch = get_model_architecture(model);
    
    // Common audio-enabled architectures
    return strstr(arch, "whisper") != nullptr ||
           strstr(arch, "audio") != nullptr ||
           strstr(arch, "speech") != nullptr;
}

} // extern "C"