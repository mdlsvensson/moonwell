using War3Net.IO.Mpq;

namespace Moonwell.Build
{
    /// <summary>Packs the staged map folder into a map archive.</summary>
    public static class Archive
    {
        /// <summary>Packs every file under the staged folder, byte for byte, with names that use <c>\</c> as the separator.</summary>
        /// <param name="stagedFolder">The staged map folder.</param>
        /// <param name="archivePath">The archive to write; replaced if it exists.</param>
        public static void Pack(string stagedFolder, string archivePath)
        {
            var streams = new List<FileStream>();
            try
            {
                var builder = new MpqArchiveBuilder();
                var files = Directory.GetFiles(stagedFolder, "*", SearchOption.AllDirectories)
                    .Select(file => Path.GetRelativePath(stagedFolder, file))
                    .OrderBy(file => file, StringComparer.Ordinal);
                foreach (var file in files)
                {
                    var stream = File.OpenRead(Path.Combine(stagedFolder, file));
                    streams.Add(stream);
                    builder.AddFile(MpqFile.New(stream, file.Replace('/', '\\')));
                }

                Directory.CreateDirectory(Path.GetDirectoryName(Path.GetFullPath(archivePath))!);
                if (File.Exists(archivePath))
                {
                    File.Delete(archivePath);
                }

                builder.SaveTo(archivePath, new MpqArchiveCreateOptions
                {
                    ListFileCreateMode = MpqFileCreateMode.Overwrite,
                    AttributesCreateMode = MpqFileCreateMode.Prune,
                    BlockSize = 3,
                });
            }
            finally
            {
                foreach (var stream in streams)
                {
                    stream.Dispose();
                }
            }
        }
    }
}
