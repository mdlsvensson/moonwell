using System.Globalization;

namespace Moonwell.Build
{
    /// <summary>The map's own gameplay constants, <c>war3mapMisc.txt</c> in the map's folder.</summary>
    public static class MapMisc
    {
        private const string FileName = "war3mapMisc.txt";
        private const string Section = "Misc";
        private const string StrengthKey = "StrHitPointBonus";
        private const string IntelligenceKey = "IntManaBonus";

        /// <summary>Reads the hero bonuses the map's file sets over the standard ones.</summary>
        /// <param name="mapFolder">The map's folder.</param>
        /// <param name="standard">The bonuses of the data set the map plays under, which a key the file leaves out keeps.</param>
        /// <returns>The map's bonuses; the standard ones when the map has no file.</returns>
        /// <exception cref="BuildException">A value of the file is not a number.</exception>
        public static HeroBonus Read(string mapFolder, HeroBonus standard)
        {
            var path = Path.Combine(mapFolder, FileName);
            if (!File.Exists(path))
            {
                return standard;
            }

            var values = new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase);
            var inSection = false;
            foreach (var raw in File.ReadAllLines(path))
            {
                var line = raw.Trim();
                if (line.Length == 0 || line.StartsWith("//", StringComparison.Ordinal))
                {
                    continue;
                }

                if (line[0] == '[' && line[line.Length - 1] == ']')
                {
                    inSection = string.Equals(line.Substring(1, line.Length - 2).Trim(), Section, StringComparison.OrdinalIgnoreCase);
                    continue;
                }

                var equals = line.IndexOf('=');
                if (inSection && equals > 0)
                {
                    values[line.Substring(0, equals).Trim()] = line.Substring(equals + 1).Trim();
                }
            }

            return new HeroBonus(
                Number(values, StrengthKey, standard.LifePerStrength, path),
                Number(values, IntelligenceKey, standard.ManaPerIntelligence, path));
        }

        private static float Number(Dictionary<string, string> values, string key, float standard, string path)
        {
            if (!values.TryGetValue(key, out var text))
            {
                return standard;
            }

            if (!float.TryParse(text, NumberStyles.Float, CultureInfo.InvariantCulture, out var number))
            {
                throw new BuildException($"{path}: {key} is '{text}', not a number");
            }

            return number;
        }
    }
}
