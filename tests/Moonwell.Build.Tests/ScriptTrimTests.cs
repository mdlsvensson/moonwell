using Xunit;

namespace Moonwell.Build.Tests
{
    /// <summary>CSharp.lua's numerics and threading files leave the script when nothing else in it names what they define.</summary>
    public sealed class ScriptTrimTests
    {
        private const string Core = "local System = System\nSystem.define(\"System.Object\", {})";
        private const string Vector = "local System = System\nSystem.defStc(\"System.Numerics.Vector2\", {})";
        private const string Task = "local System = System\nfunction System.async(f) end\nSystem.Task = System.define(\"System.Threading.Tasks.Task\", {})";

        // Each file in its own do ... end block, as CSharp.lua links it, the grouped ones between their marker lines.
        private static string Linked(bool vector = true, bool task = true, bool marked = true)
        {
            var script = Block(Core);
            if (vector)
            {
                script += Block(marked ? ScriptLibraries.Marked("Numerics", Vector) : Vector + "\n");
            }

            if (task)
            {
                script += Block(marked ? ScriptLibraries.Marked("Threading", Task) : Task + "\n");
            }

            return script;
        }

        private static string Block(string text)
        {
            return "do\n" + text.TrimEnd('\n') + "\nend\n\n";
        }

        // The line ends the linker itself writes around a file: do\r\n, then \r\nend\r\n\r\n after the file's last line.
        private static string LinkerLineEnds(string script)
        {
            return script.Replace("do\n", "do\r\n").Replace("\nend\n\n", "\n\r\nend\r\n\r\n");
        }

        [Fact]
        public void the_linkers_own_line_ends_are_read()
        {
            var compiled = "function main() end\r\n";

            Assert.Equal(LinkerLineEnds(Linked(vector: false, task: false)) + compiled, ScriptLibraries.Trim(LinkerLineEnds(Linked()) + compiled));
        }

        [Fact]
        public void unused_numerics_and_threading_leave_the_script()
        {
            var compiled = "local x = System.Object()\nfunction main() end\n";

            Assert.Equal(Linked(vector: false, task: false) + compiled, ScriptLibraries.Trim(Linked() + compiled));
        }

        [Fact]
        public void a_group_the_code_names_stays_without_its_markers()
        {
            var compiled = "local v = System.Numerics.Vector2(1, 2)\nlocal t = System.async(function () end)\n";

            Assert.Equal(Linked(marked: false) + compiled, ScriptLibraries.Trim(Linked() + compiled));
        }

        [Fact]
        public void a_short_name_the_code_names_keeps_its_group()
        {
            var compiled = "local t = System.Task.Run()\nlocal v = System.NumericsExtra.Vector2\n";

            Assert.Equal(Linked(vector: false, marked: false) + compiled, ScriptLibraries.Trim(Linked() + compiled));
        }

        [Fact]
        public void a_namespace_the_code_names_keeps_its_group()
        {
            // CSharp.lua's alias of a namespace a file uses: the full name of what it uses is not written.
            var compiled = "local SystemNumerics = System.Numerics\nlocal SystemThreadingTasks = System.Threading.Tasks\n";

            Assert.Equal(Linked(marked: false) + compiled, ScriptLibraries.Trim(Linked() + compiled));
        }
    }
}
