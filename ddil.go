package main

import (
        "archive/tar"
        "compress/gzip"
        "encoding/json"
        "io/ioutil"
        "fmt"
        "io"
        "os"
        "os/exec"
        "path/filepath"
        "strings"
)

type Manifest struct {
        Layers []string `json:"Layers"`
}

func main() {
        // 检测Docker命令是否存在
        if !isDockerInstalled() {
                fmt.Println("Docker is not installed. Please install Docker before running this program.")
                return
        }

        // 获取两个Docker镜像地址
        if len(os.Args) < 3 {
                fmt.Println("Usage: ddil <image1_url_old> <image2_url_new>")
                return
        }

        // 获取两个Docker镜像地址
        image1URL := os.Args[1]
        image2URL := os.Args[2]

        fmt.Println("Work dir .")
        fmt.Printf("Old image: %s\n", image1URL)
        fmt.Printf("New image: %s\n", image2URL)

        basePath := ".tmp/"
        os.RemoveAll(basePath)

        // 拉取和解压第一个Docker镜像
        extractDir1 := basePath + "extracted_image_old"
        os.MkdirAll(extractDir1, os.ModePerm)
        if err := pullAndExtractDockerImage(image1URL, extractDir1); err != nil {
                fmt.Printf("Error pulling or extracting Docker image 1: %s\n", err)
                return
        }

        // 拉取和解压第二个Docker镜像
        extractDir2 := basePath + "extracted_image_new"
        os.MkdirAll(extractDir2, os.ModePerm)
        if err := pullAndExtractDockerImage(image2URL, extractDir2); err != nil {
                fmt.Printf("Error pulling or extracting Docker image 2: %s\n", err)
                return
        }

        // 比较两个解压后的镜像，获取不同的层
        if err := difflayers(extractDir1, extractDir2); err != nil  {
                fmt.Printf("Error difflayers : %s\n", err)
                return
        }

        // 将不同的层压缩成一个对比压缩包
        outputFileName := strings.Replace(strings.Split(image2URL, ":")[0], "/", "_", -1) + "__" + strings.Split(image1URL, ":")[1] + "__" + strings.Split(image2URL, ":")[1] + "__diff.tar.gz"
        if err := compressLayers(extractDir2, outputFileName); err != nil {
             fmt.Printf("Error compressing diff layers: %s\n", err)
             return
        }

        if err := os.RemoveAll(".tmp"); err != nil {
                fmt.Printf("Failed to remove temporary dir: %v\n", err)
        }

        fmt.Printf("Save diff layers to: %s\n", outputFileName)
        fmt.Println("Done")
}

// 检测Docker命令是否存在
func isDockerInstalled() bool {
        cmd := exec.Command("docker", "--version")
        err := cmd.Run()
        return err == nil
}

// 拉取和解压Docker镜像
func pullAndExtractDockerImage(imageURL, extractDir string) error {
        // 拉取Docker镜像
        cmd := exec.Command("docker", "pull", imageURL)
        output, err := cmd.CombinedOutput()
        if err != nil {
                return fmt.Errorf("failed to pull Docker image: %s, error: %v", string(output), err)
        }

        // 获取镜像ID
        imageID := getImageIDFromOutput(string(output))

        // 保存Docker镜像为tar文件
        saveFilePath := imageID + ".tar"
        cmd = exec.Command("docker", "save", "-o", saveFilePath, imageURL)
        output, err = cmd.CombinedOutput()
        if err != nil {
                return fmt.Errorf("failed to save Docker image as tar: %s, error: %v", string(output), err)
        }

        // 解压Docker镜像
        err = extractTar(saveFilePath, extractDir)
        if err != nil {
                return fmt.Errorf("failed to extract Docker image: %v", err)
        }

        // 删除保存的tar文件
        if err := os.Remove(saveFilePath); err != nil {
                fmt.Printf("Failed to remove temporary tar file: %v\n", err)
        }

        return nil
}

// 获取镜像ID
func getImageIDFromOutput(output string) string {
        lines := strings.Split(output, "\n")
        if len(lines) > 0 {
                parts := strings.Split(lines[0], " ")
                if len(parts) > 2 {
                        return strings.TrimSpace(parts[2])
                }
        }
        return ""
}


func extractTar(tarFilePath, targetDir string) error {
        file, err := os.Open(tarFilePath)
        if err != nil {
                return err
        }
        defer file.Close()

        tr := tar.NewReader(file)

        for {
                header, err := tr.Next()
                if err == io.EOF {
                        break
                }

                if err != nil {
                        return err
                }

                targetFile := filepath.Join(targetDir, header.Name)

                if header.FileInfo().IsDir() {
                        err = os.MkdirAll(targetFile, header.FileInfo().Mode())
                        if err != nil {
                                return err
                        }
                } else {
                        f, err := os.OpenFile(targetFile, os.O_CREATE|os.O_RDWR, header.FileInfo().Mode())
                        if err != nil {
                                return err
                        }
                        defer f.Close()

                        _, err = io.Copy(f, tr)
                        if err != nil {
                                return err
                        }
                }
        }

        return nil
}


// 对比两个镜像不同的层
func difflayers(dir1, dir2 string)  error {

        layersA, err := readManifestLayers(filepath.Join(dir1, "manifest.json"))
        if err != nil {
                fmt.Printf("Error reading manifest.json in directory A: %s\n", err)
                return err
        }

        layersB, err := readManifestLayers(filepath.Join(dir2, "manifest.json"))
        if err != nil {
                fmt.Printf("Error reading manifest.json in directory B: %s\n", err)
                return err
        }

        existlayers, err := os.Create(filepath.Join(dir2, "existlayers"))
        if err != nil {
                fmt.Printf("Error creating existlayers file: %s\n", err)
                return err
        }
        defer existlayers.Close()

        for _, layer := range layersA {
                if contains(layersB, layer) {
                        layerFile := filepath.Join(dir2, layer)
                        layerDir := strings.Replace(layerFile, "/layer.tar", "", -1)
                        layerDirArray := strings.Split(layerDir, "/")
                        layerID := layerDirArray[len(layerDirArray) - 1]

                        err := os.RemoveAll(layerDir)
                        if err != nil {
                                fmt.Printf("Error deleting directory %s: %s\n", layerID, err)
                        } else {
                                fmt.Printf("ExistLayers %s deleted.\n", layerID)
                        }
                        
                        _, err = existlayers.WriteString(layerID)
                        if err != nil {
                                fmt.Printf("Error writing to existlayers file: %s\n", err)
                        }
                }
        }

        return nil

}

func readManifestLayers(manifestPath string) ([]string, error) {
        var manifests []Manifest

        file, err := ioutil.ReadFile(manifestPath)
        if err != nil {
                return nil, err
        }

        err = json.Unmarshal(file, &manifests)
        if err != nil {
                return nil, err
        }

        if len(manifests) == 0 {
                return nil, fmt.Errorf("no manifests found in the file")
        }

        return manifests[0].Layers, nil
}

func contains(slice []string, str string) bool {
        for _, s := range slice {
                if s == str {
                        return true
                }
        }
        return false
}


func compressLayers(compressDir string, outputFileName string) error {
        // Create the output tar.gz file
        tarGzFile, err := os.Create(outputFileName)
        if err != nil {
                fmt.Printf("Error creating tar.gz file: %s\n", err)
                return err
        }
        defer tarGzFile.Close()

        // Create the gzip writer
        gzipWriter := gzip.NewWriter(tarGzFile)
        defer gzipWriter.Close()

        // Create the tar writer
        tarWriter := tar.NewWriter(gzipWriter)
        defer tarWriter.Close()

        // Walk through the source directory and add all its contents to the tar.gz
        err = filepath.Walk(compressDir, func(path string, info os.FileInfo, err error) error {
                if err != nil {
                        return err
                }

                // Skip the root directory itself (A directory)
                if path == compressDir {
                        return nil
                }

                // Create a tar header
                relPath, err := filepath.Rel(compressDir, path)
                if err != nil {
                        return err
                }

                header, err := tar.FileInfoHeader(info, "")
                if err != nil {
                        return err
                }
                header.Name = relPath

                // Write the tar header
                if err := tarWriter.WriteHeader(header); err != nil {
                        return err
                }

                // If it's a file, copy the contents into the tar.gz
                if !info.IsDir() {
                        file, err := os.Open(path)
                        if err != nil {
                                return err
                        }
                        defer file.Close()

                        _, err = io.Copy(tarWriter, file)
                        if err != nil {
                                return err
                        }
                }

                return nil
        })

        if err != nil {
                fmt.Printf("Error compressing directory: %s\n", err)
                return err
        }

        fmt.Println("Images compressed successfully.")

        return nil


}
