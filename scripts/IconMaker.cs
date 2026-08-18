using System;
using System.Drawing;
using System.Drawing.Imaging;
using System.IO;
using System.Linq;

class IconMaker
{
    static void Main(string[] args)
    {
        string inputPath = args[0];
        string outputDir = args[1];
        Directory.CreateDirectory(outputDir);

        using (Image original = Image.FromFile(inputPath))
        {
            int size = Math.Min(original.Width, original.Height);
            using (Bitmap square = new Bitmap(size, size, System.Drawing.Imaging.PixelFormat.Format32bppArgb))
            {
                using (Graphics g = Graphics.FromImage(square))
                {
                    g.DrawImage(original, 0, 0, size, size);
                }

                // Save PNG sizes
                int[] pngSizes = { 16, 32, 64, 128, 256, 512 };
                foreach (int s in pngSizes)
                {
                    using (Bitmap bmp = new Bitmap(s, s, System.Drawing.Imaging.PixelFormat.Format32bppArgb))
                    {
                        using (Graphics g = Graphics.FromImage(bmp))
                        {
                            g.InterpolationMode = System.Drawing.Drawing2D.InterpolationMode.HighQualityBicubic;
                            g.SmoothingMode = System.Drawing.Drawing2D.SmoothingMode.HighQuality;
                            g.DrawImage(square, 0, 0, s, s);
                        }
                        bmp.Save(Path.Combine(outputDir, $"snowden_system_{s}x{s}.png"), ImageFormat.Png);
                    }
                }

                // Create ICO with multiple sizes
                int[] icoSizes = { 16, 24, 32, 48, 64, 96, 128, 256 };
                using (MemoryStream ms = new MemoryStream())
                {
                    WriteIco(ms, square, icoSizes);
                    File.WriteAllBytes(Path.Combine(outputDir, "snowden_system_icon.ico"), ms.ToArray());
                }
            }
        }
        Console.WriteLine("Done");
    }

    static void WriteIco(Stream stream, Bitmap source, int[] sizes)
    {
        BinaryWriter writer = new BinaryWriter(stream);
        // ICO header
        writer.Write((short)0); // Reserved
        writer.Write((short)1); // Type: icon
        writer.Write((short)sizes.Length); // Count

        long dirOffset = stream.Position;
        int imageDataOffset = 6 + sizes.Length * 16;
        int currentOffset = imageDataOffset;

        foreach (int size in sizes)
        {
            using (Bitmap bmp = new Bitmap(size, size, System.Drawing.Imaging.PixelFormat.Format32bppArgb))
            {
                using (Graphics g = Graphics.FromImage(bmp))
                {
                    g.InterpolationMode = System.Drawing.Drawing2D.InterpolationMode.HighQualityBicubic;
                    g.SmoothingMode = System.Drawing.Drawing2D.SmoothingMode.HighQuality;
                    g.DrawImage(source, 0, 0, size, size);
                }

                MemoryStream pngStream = new MemoryStream();
                bmp.Save(pngStream, ImageFormat.Png);
                byte[] pngData = pngStream.ToArray();
                int dataSize = pngData.Length;

                writer.Write((byte)(size == 256 ? 0 : size)); // Width
                writer.Write((byte)(size == 256 ? 0 : size)); // Height
                writer.Write((byte)0); // Colors
                writer.Write((byte)0); // Reserved
                writer.Write((short)1); // Color planes
                writer.Write((short)32); // Bits per pixel
                writer.Write(dataSize); // Size of image data
                writer.Write(currentOffset); // Offset to image data
                currentOffset += dataSize;
            }
        }

        foreach (int size in sizes)
        {
            using (Bitmap bmp = new Bitmap(size, size, System.Drawing.Imaging.PixelFormat.Format32bppArgb))
            {
                using (Graphics g = Graphics.FromImage(bmp))
                {
                    g.InterpolationMode = System.Drawing.Drawing2D.InterpolationMode.HighQualityBicubic;
                    g.SmoothingMode = System.Drawing.Drawing2D.SmoothingMode.HighQuality;
                    g.DrawImage(source, 0, 0, size, size);
                }
                bmp.Save(stream, ImageFormat.Png);
            }
        }
        writer.Flush();
    }
}
