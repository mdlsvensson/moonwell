namespace Moonwell.Build
{
    /// <summary>Builds a map project into a map archive.</summary>
    public static class MapBuild
    {
        /// <summary>Runs every step of the build, in order.</summary>
        /// <param name="path">A project folder, or the path of its <c>.csproj</c> file.</param>
        /// <param name="options">What kind of build to make.</param>
        /// <param name="log">Where the build's progress, notes and <c>dotnet build</c> output go.</param>
        /// <returns>The full path of the map archive.</returns>
        /// <exception cref="BuildException">A step failed in a way the modder can act on.</exception>
        public static string Run(string path, BuildOptions options, TextWriter log)
        {
            var project = MapProject.Find(path);
            log.WriteLine($"moonwell: {project.Name} ({(options.Release ? "release" : "development")} build)");

            var targetPath = DotnetBuild.Run(project, options.Release, log);
            PackageSources.Check(project);
            var settings = MapSettingsReader.Read(targetPath, project);
            var map = SourceMap.Open(project, settings);
            EntryWriter.Write(project, settings);
            LuaCompiler.Compile(project, map, targetPath, options.Release);

            try
            {
                Stage.Write(project, map, log);
            }
            catch (Exception error) when (error is IOException or UnauthorizedAccessException)
            {
                throw CannotWrite(project, project.StagedFolder, error);
            }

            try
            {
                Archive.Pack(project.StagedFolder, project.ArchivePath);
            }
            catch (Exception error) when (error is IOException or UnauthorizedAccessException)
            {
                throw CannotWrite(project, project.ArchivePath, error);
            }

            log.WriteLine($@"moonwell: built build\{project.Name}.w3x");
            return project.ArchivePath;
        }

        private static BuildException CannotWrite(MapProject project, string path, Exception error)
        {
            var relative = Path.GetRelativePath(project.Folder, path);
            return new BuildException($"cannot write {relative}: {error.Message} (is the game or World Editor using it?)");
        }
    }
}
