package main

import (
    "fmt"
    "io"
    "mime"
    "net/http"
    "os"
    "path/filepath"

    "github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
    "github.com/google/uuid"
)

func getFileExtension(mediaType string) string {
    switch mediaType {
    case "image/jpeg":
        return "jpg"
    case "image/png":
        return "png"
    case "image/gif":
        return "gif"
    default:
        return "bin"
    }
}

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
    videoIDString := r.PathValue("videoID")
    videoID, err := uuid.Parse(videoIDString)
    if err != nil {
        respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
        return
    }

    token, err := auth.GetBearerToken(r.Header)
    if err != nil {
        respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
        return
    }

    userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
    if err != nil {
        respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
        return
    }

    const maxMemory = 10 << 20 // 10 MB
    r.ParseMultipartForm(maxMemory)

    file, header, err := r.FormFile("thumbnail")
    if err != nil {
        respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
        return
    }
    defer file.Close()

    contentType := header.Header.Get("Content-Type")
    if contentType == "" {
        respondWithError(w, http.StatusBadRequest, "Missing Content-Type for thumbnail", nil)
        return
    }

    mediaType, _, err := mime.ParseMediaType(contentType)
    if err != nil {
        respondWithError(w, http.StatusBadRequest, "Invalid Content-Type for thumbnail", err)
        return
    }

	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Only JPEG and PNG thumbnails are allowed", err)
		return
	}

    video, err := cfg.db.GetVideo(videoID)
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Couldn't find video", err)
        return
    }

    if video.UserID != userID {
        respondWithError(w, http.StatusUnauthorized, "Not authorized to update this video", nil)
        return
    }

    // Determine file extension and path
    ext := getFileExtension(mediaType)
    filename := fmt.Sprintf("%s.%s", videoID.String(), ext)
    filePath := filepath.Join(cfg.assetsRoot, filename)

    // Save file to disk
    outFile, err := os.Create(filePath)
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Could not create file", err)
        return
    }
    defer outFile.Close()

    // Copy file contents to disk
    if _, err := io.Copy(outFile, file); err != nil {
        respondWithError(w, http.StatusInternalServerError, "Could not save file", err)
        return
    }

    // Set the thumbnail_url to the asset path (adjust as needed for your server setup)
    thumbnailURL := fmt.Sprintf("/assets/%s", filename)
    video.ThumbnailURL = &thumbnailURL

    err = cfg.db.UpdateVideo(video)
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
        return
    }

    respondWithJSON(w, http.StatusOK, video)
}
