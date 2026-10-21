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

        [Fact]
        public void the_check_names_ints_made_floats_and_bare_float_fields()
        {
            var folder = Path.Combine(Path.GetTempPath(), "moonwell-package-floats-" + Guid.NewGuid().ToString("N"));
            Directory.CreateDirectory(folder);
            try
            {
                var file = Path.Combine(folder, "Bare.cs");
                File.WriteAllText(file, string.Join("\n", new[]
                {
                    "public class Bare",
                    "{",
                    "    public float Value;",
                    "    public static double Auto { get; set; }",
                    "    public static float Of(int count) { return (float)count + count * 2; }",
                    "    public static float Written(int count) { return (float)count + 0.0f; }",
                    "}",
                }));

                var (changes, errors) = FloatLiterals.Check(new[] { file }, Array.Empty<string>());

                Assert.Empty(errors);
                Assert.Equal(
                    new[]
                    {
                        file + "(3): Value -> Value = 0.0f",
                        file + "(4): { get; set; } -> { get; set; } = 0.0",
                        file + "(5): (float)count -> ((float)count + 0.0f)",
                    },
                    changes);
            }
            finally
            {
                Directory.Delete(folder, true);
            }
        }
    }
}
