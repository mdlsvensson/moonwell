using Xunit;

namespace Moonwell.Build.Tests
{
    /// <summary>
    /// Moonwell's own source reaches the map from its packages, not from the marked copy the build rewrites (C18), so it is
    /// written with points: no whole-number float or double constant CSharp.lua would give Lua as an integer.
    /// </summary>
    public sealed class PackageFloatsTests
    {
        // The projects CSharp.lua compiles into a map, as it does: from source, together.
        private static readonly string[] LuaProjects = { "Moonwell.Natives", "Moonwell", "Moonwell.Objects", "Moonwell.Damage" };

        [Theory]
        [InlineData("DEBUG", "MOONWELL_LUA")]
        [InlineData("MOONWELL_LUA")]
        public void packages_write_whole_number_floats_with_a_point(params string[] symbols)
        {
            var src = Path.Combine(TestFolders.Fixture("."), "..", "..", "..", "src");
            var files = LuaProjects
                .SelectMany(project => Directory.EnumerateFiles(Path.GetFullPath(Path.Combine(src, project)), "*.cs", SearchOption.AllDirectories))
                .Where(path => !path.Contains(Path.DirectorySeparatorChar + "bin" + Path.DirectorySeparatorChar, StringComparison.OrdinalIgnoreCase)
                    && !path.Contains(Path.DirectorySeparatorChar + "obj" + Path.DirectorySeparatorChar, StringComparison.OrdinalIgnoreCase))
                .ToList();

            var (changes, errors) = FloatLiterals.Check(files, symbols);

            Assert.True(errors.Count == 0, string.Join(Environment.NewLine, errors));
            Assert.True(changes.Count == 0, string.Join(Environment.NewLine, changes));
        }
    }
}
