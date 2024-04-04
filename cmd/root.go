package cmd

import (
	"log/slog"
	"vimeo-archive/app"
	"vimeo-archive/lib/model"
	libvimeo "vimeo-archive/lib/vimeo"

	"github.com/defval/di"
	"github.com/silentsokolov/go-vimeo/v2/vimeo"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func Bootstrap(c *app.AppContainer) error {
	archiveCmd := &cobra.Command{
		Use:   "archive",
		Short: "Archive Vimeo videos",
		RunE: c.RunE(func(vc *vimeo.Client, vb *model.VideoBox) error {
			va := libvimeo.NewArchiver(
				libvimeo.WithVimeoClient(vc),
				libvimeo.WithVideoBox(vb),
				libvimeo.WithMax(viper.GetUint64("max")),
			)

			return va.Archive()
		}),
	}

	archiveCmd.Flags().Int("max", 0, "Maximum number of videos to archive")
	viper.BindPFlag("max", archiveCmd.Flags().Lookup("max"))

	scrapeCmd := &cobra.Command{
		Use:   "scrape",
		Short: "Scrape Vimeo videos into the Objectbox Database",
		RunE: c.RunE(func(vc *vimeo.Client, vb *model.VideoBox) error {
			vs := libvimeo.NewScraper(
				libvimeo.WithAPI(vc),
				libvimeo.WithPageSize(viper.GetInt("page-size")),
				libvimeo.WithPagePointer(viper.GetInt("page-pointer")),
				libvimeo.WithMaxPages(viper.GetInt("max-pages")),
			)

			for vs.HasNextPage() {
				videos, err := vs.ListVideos()
				if err != nil {
					slog.Error("unable to list videos", "error", err)
					return err
				}

				slog.Debug("listed vimeo videos", "count", len(videos))

				for _, v := range videos {
					video := model.VideoFromVimeo(v)
					_, err := vb.Put(video)
					if err != nil {
						slog.Error("unable to put video", "error", err)
						return err
					}
				}
			}

			return nil
		}),
	}

	scrapeCmd.Flags().Int("page-size", 25, "Number of videos to fetch per page")
	viper.BindPFlag("page-size", scrapeCmd.Flags().Lookup("page-size"))
	scrapeCmd.Flags().Int("page-pointer", 0, "Page pointer to start fetching videos")
	viper.BindPFlag("page-pointer", scrapeCmd.Flags().Lookup("page-pointer"))
	scrapeCmd.Flags().Int("max-pages", 0, "Maximum number of pages to fetch")
	viper.BindPFlag("max-pages", scrapeCmd.Flags().Lookup("max-pages"))

	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show statistics of the Vimeo videos",
		RunE: c.RunE(func(vb *model.VideoBox) error {
			total, err := vb.Count()
			if err != nil {
				slog.Error("unable to count videos", "error", err)
				return err
			}

			nonMuxed, err := vb.Query(
				model.Video_.Id.NotIn(muxed...),
			).Count()
			if err != nil {
				slog.Error("unable to count non-muxed videos", "error", err)
				return err
			}

			downloaded, err := vb.Query(
				model.Video_.DownloadedTime.IsNotNil(),
			).Count()
			if err != nil {
				slog.Error("unable to count downloaded videos", "error", err)
				return err
			}

			slog.Info("stats",
				"total", total,
				"muxed", len(muxed),
				"non-muxed", nonMuxed,
				"downloaded", downloaded,
			)

			return nil
		}),
	}

	rootCmd := &cobra.Command{
		Use:   "vimeo-archiver",
		Short: "Vimeo Archiver is a tool to archive Vimeo videos",
	}

	rootCmd.AddCommand(archiveCmd)
	rootCmd.AddCommand(scrapeCmd)
	rootCmd.AddCommand(statsCmd)

	return c.Apply(
		di.ProvideValue(rootCmd),
	)
}
