#include "binding.h"
#include "whisper.h"
#include <string>
#include <vector>
#include <cstring>

extern "C" {

void* whisper_load_model_from_file(const char *fname) {
    struct whisper_context_params cparams = whisper_context_default_params();
    return whisper_init_from_file_with_params(fname, cparams);
}

void whisper_free_model_wrapper(void* model) {
    if (model) {
        whisper_free((struct whisper_context*)model);
    }
}

int whisper_transcribe_wrapper(void* ctx, const float* audio_data, int audio_len, 
                              const char* language, char* result, int result_size,
                              float* segment_starts, float* segment_ends, char** segment_texts, int max_segments) {
    if (!ctx) return -1;
    
    struct whisper_context* wctx = (struct whisper_context*)ctx;
    
    // Set up parameters
    struct whisper_full_params wparams = whisper_full_default_params(WHISPER_SAMPLING_GREEDY);
    if (language && strlen(language) > 0 && strcmp(language, "auto") != 0) {
        wparams.language = language;
    }
    wparams.translate = false;
    wparams.print_realtime = false;
    wparams.print_progress = false;
    wparams.print_timestamps = false;
    wparams.print_special = false;
    
    // Run transcription
    if (whisper_full(wctx, wparams, audio_data, audio_len) != 0) {
        return -1;
    }
    
    // Get number of segments
    const int n_segments = whisper_full_n_segments(wctx);
    
    // Build result text
    std::string full_text;
    int segments_written = 0;
    
    for (int i = 0; i < n_segments && segments_written < max_segments; ++i) {
        const char* text = whisper_full_get_segment_text(wctx, i);
        const int64_t t0 = whisper_full_get_segment_t0(wctx, i);
        const int64_t t1 = whisper_full_get_segment_t1(wctx, i);
        
        // Add to full text
        if (i > 0) full_text += " ";
        full_text += text;
        
        // Add segment data
        if (segment_starts && segment_ends && segment_texts) {
            segment_starts[segments_written] = t0 * 0.01f; // Convert to seconds
            segment_ends[segments_written] = t1 * 0.01f;
            segment_texts[segments_written] = strdup(text);
            segments_written++;
        }
    }
    
    // Copy result text
    if (result && result_size > 0) {
        strncpy(result, full_text.c_str(), result_size - 1);
        result[result_size - 1] = '\0';
    }
    
    return segments_written;
}

int whisper_get_supported_languages_wrapper(void* model, char** languages, int max_languages) {
    // Return common languages supported by Whisper
    const char* supported[] = {"en", "de", "fr", "es", "it", "pt", "ru", "ja", "ko", "zh", "auto"};
    int count = sizeof(supported) / sizeof(supported[0]);
    
    if (count > max_languages) count = max_languages;
    
    for (int i = 0; i < count; i++) {
        languages[i] = strdup(supported[i]);
    }
    
    return count;
}

int whisper_is_multilingual_wrapper(void* model) {
    if (!model) return 0;
    struct whisper_context* wctx = (struct whisper_context*)model;
    return whisper_is_multilingual(wctx);
}

} // extern "C"