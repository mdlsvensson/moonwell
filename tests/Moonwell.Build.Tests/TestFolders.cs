namespace Moonwell.Build.Tests
{
    /// <summary>Finds the fixtures and keeps temporary copies of them, deleted when the test is done.</summary>
    public sealed class TestFolders : IDisposable
    {
        private readonly List<string> folders = new List<string>();

        /// <summary>The full path of a fixture folder; <c>.</c> is the folder of all fixtures.</summary>
        public static string Fixture(string name)
        {
            return Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "Fixtures", name));
        }

        /// <summary>Copies the Sample, without its bin, obj and build folders, beside the other fixtures so its relative reference to Moonwell holds.</summary>
        public string TemporaryCopyOfSample()
        {
            return TemporaryCopyOf("Sample");
        }

        /// <summary>Copies a fixture, without its bin, obj and build folders, beside the other fixtures so its relative references hold.</summary>
        public string TemporaryCopyOf(string fixture)
        {
            var folder = Path.Combine(Fixture("."), "Temp-" + Guid.NewGuid().ToString("N"));
            folders.Add(folder);
            CopyFolder(Fixture(fixture), folder);
            return folder;
        }

        /// <summary>Copies the Sample, without its bin, obj and build folders, into the system's temporary folder, outside the repository.</summary>
        public string TemporaryCopyOfSampleOutsideRepository()
        {
            var folder = Path.Combine(Path.GetTempPath(), "moonwell-packaged-" + Guid.NewGuid().ToString("N"));
            folders.Add(folder);
            CopyFolder(Fixture("Sample"), folder);
            return folder;
        }

        /// <summary>An empty folder in the system's temporary folder, outside the repository.</summary>
        public string TemporaryFolderOutsideRepository()
        {
            var folder = Path.Combine(Path.GetTempPath(), "moonwell-empty-" + Guid.NewGuid().ToString("N"));
            folders.Add(folder);
            Directory.CreateDirectory(folder);
            return folder;
        }

        /// <summary>Deletes the temporary copies.</summary>
        public void Dispose()
        {
            foreach (var folder in folders)
            {
                Delete(folder);
            }
        }

        /// <summary>
        /// Deletes a temporary folder of a test's own, and what is in it, if it is there. On Windows a process the test started
        /// (a build server, an MSBuild node) may still hold a file in it for a moment: the delete is tried again for two seconds,
        /// then the folder is left, since a leftover temporary folder is not a test's failure.
        /// </summary>
        public static void Delete(string folder, bool recursive = true)
        {
            for (var attempt = 1; Directory.Exists(folder); attempt++)
            {
                try
                {
                    Directory.Delete(folder, recursive);
                    return;
                }
                catch (Exception error) when (error is IOException or UnauthorizedAccessException)
                {
                    if (attempt == 20)
                    {
                        return;
                    }

                    Thread.Sleep(100);
                }
            }
        }

        private static void CopyFolder(string from, string to)
        {
            Directory.CreateDirectory(to);
            foreach (var file in Directory.GetFiles(from))
            {
                File.Copy(file, Path.Combine(to, Path.GetFileName(file)));
            }

            foreach (var directory in Directory.GetDirectories(from))
            {
                var name = Path.GetFileName(directory);
                if (name is "bin" or "obj" or "build")
                {
                    continue;
                }

                CopyFolder(directory, Path.Combine(to, name));
            }
        }
    }
}
