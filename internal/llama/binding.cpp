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
    
    // Main generation loop following simple.cpp exactly
    llama_batch batch = llama_batch_get_one(prompt_tokens.data(), prompt_tokens.size());
    int n_pos = 0;
    
    for (int i = 0; i < max_tokens && n_pos + batch.n_tokens < n_prompt + max_tokens; ) {
        // Decode current batch
        if (llama_decode(context, batch)) {
            break; // Failed to evaluate
        }
        
        n_pos += batch.n_tokens;
        
        // Sample next token
        llama_token new_token_id = llama_sampler_sample(smpl, context, -1);
        
        // Check for end of generation
        if (llama_vocab_is_eog(vocab, new_token_id)) {
            break;
        }
        
        // Convert token to text
        char buf[128];
        int n = llama_token_to_piece(vocab, new_token_id, buf, sizeof(buf), 0, true);
        if (n > 0) {
            generated_text.append(buf, n);
            tokens_generated++;
        }
        
        // Accept token for sampler state management
        llama_sampler_accept(smpl, new_token_id);
        
        // Prepare next batch with the new token
        batch = llama_batch_get_one(&new_token_id, 1);
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

} // extern "C"