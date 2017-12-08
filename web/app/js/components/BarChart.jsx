import _ from 'lodash';
import { metricToFormatter } from './util/Utils.js';
import React from 'react';
import * as d3 from 'd3';
import './../../css/bar-chart.css';

const defaultSvgWidth = 595;
const defaultSvgHeight = 150;
const margin = { top: 0, right: 0, bottom: 20, left: 0 };

export default class LineGraph extends React.Component {
  constructor(props) {
    super(props);

    this.state = this.getChartDimensions();
  }

  componentWillMount() {
    this.initializeScales();
  }

  componentDidMount() {
    this.svg = d3.select("." + this.props.containerClassName)
      .append("svg")
        .attr("class", "bar-chart")
        .attr("width", this.state.svgWidth)
        .attr("height", this.state.svgHeight)
      .append("g")
        .attr("transform", "translate(" + this.state.margin.left + "," + this.state.margin.top + ")");

      this.tooltip = d3.select("." + this.props.containerClassName + " .bar-chart-tooltip")
        .append("div").attr("class", "tooltip");

    this.xAxis = this.svg.append("g")
      .attr("class", "x-axis")
      .attr("transform", "translate(0," + this.state.height + ")");

    this.updateScales();
    this.renderGraph();
  }

  shouldComponentUpdate(nextProps) {
    if (nextProps.lastUpdated === this.props.lastUpdated) {
      // control whether react re-renders the component
      // only rerender if the input data has changed
      return false;
    }
    return true;
  }

  componentDidUpdate() {
    this.updateScales();
    this.renderGraph();
  }

  getChartDimensions() {
    let svgWidth = this.props.width || defaultSvgWidth;
    let svgHeight = this.props.height || defaultSvgHeight;

    let width = svgWidth - margin.left - margin.right;
    let height = svgHeight - margin.top - margin.bottom;

    return {
      svgWidth: svgWidth,
      svgHeight: svgHeight,
      width: width,
      height: height,
      margin: margin
    };
  }

  updateScales() {
    let data = this.props.data;
    this.xScale.domain(_.map(data, d => d.name));
    this.yScale.domain([0, d3.max(data, d => d.rollup.requestRate)]);
  }

  initializeScales() {
    this.xScale = d3.scaleBand()
      .range([0, this.state.width])
      .padding(0.1);
    this.yScale = d3.scaleLinear()
      .range([this.state.height, 0]);
  }

  renderGraph() {
    let barChart = this.svg.selectAll(".bar")
      .remove()
      .exit()
      .data(this.props.data);

    barChart.enter().append("rect")
      .attr("class", "bar")
      .attr("x", d => this.xScale(d.name))
      .attr("width", () =>  this.xScale.bandwidth())
      .attr("y", d => this.yScale(d.rollup.requestRate))
      .attr("height", d => this.state.height - this.yScale(d.rollup.requestRate))
      .on("mousemove", d => {
        this.tooltip
          .style("left", d3.event.pageX - 50 + "px")
          .style("top", d3.event.pageY - 70 + "px")
          .style("display", "inline-block") // show tooltip
          .text(`${d.name}: ${metricToFormatter["REQUEST_RATE"](d.rollup.requestRate)} (${d.pretty} of total)`);
      })
      .on("mouseout", () => this.tooltip.style("display", "none"));

    this.updateAxes();
  }

  updateAxes() {
    this.xAxis
      .call(d3.axisBottom(this.xScale)); // add x axis labels
  }

  render() {
    return null;
  }
}
