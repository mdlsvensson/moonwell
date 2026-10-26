namespace Moonwell.Build
{
    /// <summary>What a build is asked to do.</summary>
    public sealed class BuildOptions
    {
        /// <summary>True for a release build (RELEASE in place of DEBUG, comments removed); false for a development build.</summary>
        public bool Release { get; init; }

        /// <summary>
        /// The version the map's Moonwell packages must have: null for this build's own, <see cref="PackageSources.ToolVersion"/>.
        /// The tests set it for the packages they pack with a version of their own.
        /// </summary>
        internal string? PackageVersion { get; init; }
    }
}
