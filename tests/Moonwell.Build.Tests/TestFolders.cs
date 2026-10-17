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
            var folder = Path.Combine(Fixture("."), "Temp-" + Guid.NewGuid().ToString("N"));
            folders.Add(folder);
            CopyFolder(Fixture("Sample"), folder);
            return folder;
        }

        /// <summary>Deletes the temporary copies.</summary>
        public void Dispose()
        {
            foreach (var folder in folders)
            {
                if (Directory.Exists(folder))
                {
                    Directory.Delete(folder, true);
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
