using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;

namespace Moonwell.Build.Tests
{
    /// <summary>What the map's built assembly refers to, read from its metadata.</summary>
    public sealed class DeclarationsReferencesTests : IDisposable
    {
        private readonly TestFolders folders;

        public DeclarationsReferencesTests(ITestOutputHelper output)
        {
            folders = new TestFolders(output);
        }

        public void Dispose()
        {
            folders.Dispose();
        }

        [Fact]
        public void references_type_finds_mouse()
        {
            var path = Compile("class A { void F(Moonwell.Player player) { var mouse = Moonwell.InputExtensions.Mouse(player); } }");

            Assert.True(Declarations.ReferencesType(path, "Moonwell", "Mouse"));
            Assert.False(Declarations.ReferencesType(path, "Moonwell", "IMouseWatch"));
            Assert.False(Declarations.ReferencesType(path, "Other", "Mouse"));
        }

        [Fact]
        public void a_call_through_the_extension_method_is_found()
        {
            var path = Compile("using Moonwell; class A { void F(Player player) { player.Mouse().ShowCursor(false); } }");

            Assert.True(Declarations.ReferencesType(path, "Moonwell", "Mouse"));
        }

        [Fact]
        public void references_type_finds_a_watch()
        {
            var path = Compile("using Moonwell; class A : IMouseWatch { }");

            Assert.True(Declarations.ReferencesType(path, "Moonwell", "IMouseWatch"));
        }

        [Fact]
        public void a_map_that_only_uses_the_camera_does_not_reference_the_mouse()
        {
            var path = Compile("using Moonwell; class A { void F(Player player) { var camera = player.Camera(); } }");

            Assert.False(Declarations.ReferencesType(path, "Moonwell", "Mouse"));
            Assert.False(Declarations.ReferencesType(path, "Moonwell", "IMouseWatch"));
            Assert.True(Declarations.ReferencesType(path, "Moonwell", "Camera"));
        }

        // Compiles a map's code against the built Moonwell.Input and writes the assembly into a folder outside the repository.
        private string Compile(string source)
        {
            var references = new List<MetadataReference>();
            foreach (var path in ((string)AppContext.GetData("TRUSTED_PLATFORM_ASSEMBLIES")!).Split(Path.PathSeparator))
            {
                if (!Path.GetFileName(path).StartsWith("Moonwell", StringComparison.Ordinal))
                {
                    references.Add(MetadataReference.CreateFromFile(path));
                }
            }

            foreach (var name in new[] { "Moonwell.dll", "Moonwell.Natives.dll", "Moonwell.Input.dll" })
            {
                references.Add(MetadataReference.CreateFromFile(Path.Combine(AppContext.BaseDirectory, name)));
            }

            var compilation = CSharpCompilation.Create(
                "MapCode",
                new[] { CSharpSyntaxTree.ParseText(source) },
                references,
                new CSharpCompilationOptions(OutputKind.DynamicallyLinkedLibrary));
            var folder = folders.TemporaryFolderOutsideRepository();
            var assembly = Path.Combine(folder, "MapCode.dll");
            var result = compilation.Emit(assembly);
            Assert.True(result.Success, string.Join("\n", result.Diagnostics.Where(d => d.Severity == DiagnosticSeverity.Error)));
            return assembly;
        }
    }
}
