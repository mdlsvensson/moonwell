using System.Text;
using System.Text.RegularExpressions;

namespace Moonwell.Build
{
    /// <summary>Turns the <see cref="LineMarks"/> markers in a compiled script into the table the runtime finds C# lines in.</summary>
    public static class LineTable
    {
        private static readonly Regex Marker = new Regex(@"^--@mw (?:(\d+):(\d+)|-)$", RegexOptions.CultureInvariant);

        /// <summary>
        /// Drops every line that is only a marker and appends, after the last line,
        /// <c>MoonwellLines = { files = { ... }, map = "..." }</c>: <c>map</c> holds a triple per marker, in script order — the line
        /// of the script the marker stood before, as the difference from the previous triple's, the file's number (0 for none)
        /// and the C# line (0 for none). A script line belongs to the nearest triple at or above it.
        /// </summary>
        /// <param name="script">The compiled script; its line ends are kept.</param>
        /// <param name="files">The marked files, relative to the project folder: the first is file 1.</param>
        /// <returns>The script without markers and with the table.</returns>
        public static string Apply(string script, IReadOnlyList<string> files)
        {
            var newLine = script.Contains("\r\n", StringComparison.Ordinal) ? "\r\n" : "\n";
            var kept = new StringBuilder(script.Length);
            var entries = new List<(int Line, int File, int CsLine)>();
            (int File, int CsLine)? pending = null;
            var line = 0;
            var start = 0;
            while (start < script.Length)
            {
                var end = script.IndexOf('\n', start);
                var next = end < 0 ? script.Length : end + 1;
                var match = Marker.Match(script[start..next].Trim());
                if (match.Success)
                {
                    pending = match.Groups[1].Success
                        ? (int.Parse(match.Groups[1].Value, System.Globalization.CultureInfo.InvariantCulture), int.Parse(match.Groups[2].Value, System.Globalization.CultureInfo.InvariantCulture))
                        : (0, 0);
                }
                else
                {
                    line++;
                    kept.Append(script, start, next - start);
                    if (pending is { } entry)
                    {
                        entries.Add((line, entry.File, entry.CsLine));
                        pending = null;
                    }
                }

                start = next;
            }

            if (kept.Length > 0 && kept[^1] != '\n')
            {
                kept.Append(newLine);
            }

            kept.Append(newLine).Append(Table(files, entries)).Append(newLine);
            return kept.ToString();
        }

        private static string Table(IReadOnlyList<string> files, List<(int Line, int File, int CsLine)> entries)
        {
            var map = new StringBuilder();
            var previous = 0;
            foreach (var entry in entries)
            {
                if (map.Length > 0)
                {
                    map.Append(' ');
                }

                map.Append(entry.Line - previous).Append(' ').Append(entry.File).Append(' ').Append(entry.CsLine);
                previous = entry.Line;
            }

            var names = string.Join(", ", files.Select(file => "\"" + file.Replace("\\", "\\\\").Replace("\"", "\\\"") + "\""));
            return "MoonwellLines = { files = { " + names + " }, map = \"" + map + "\" }";
        }
    }
}
